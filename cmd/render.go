package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/cmd/crank/render"
)

// NewRenderCommand creates a new render command.
func NewRenderCommand() *cobra.Command {
	cmd := &renderCmd{
		fs: afero.NewOsFs(),
	}

	cobraCmd := &cobra.Command{
		Use:   "render <composite-resource> <composition> [functions]",
		Short: "Render a Crossplane composition using composition functions",
		Long: `Render shows you what composed resources Crossplane would create by
printing them to stdout. It also prints any changes that would be made to the
status of the XR. It doesn't talk to Crossplane. Instead it runs the Composition
Function pipeline specified by the Composition locally, and uses that to render
the XR. It only supports Compositions in Pipeline mode.

If the functions argument is not provided, crossbench will automatically extract
function references from the composition's pipeline and use them.

Composition Functions are pulled and run using Docker by default. You can add
the following annotations to each Function to change how they're run:

  render.crossplane.io/runtime: "Development"

    Connect to a Function that is already running, instead of using Docker. This
	is useful to develop and debug new Functions. The Function must be listening
	at localhost:9443 and running with the --insecure flag.

  render.crossplane.io/runtime-development-target: "dns:///example.org:7443"

    Connect to a Function running somewhere other than localhost:9443. The
	target uses gRPC target syntax.

  render.crossplane.io/runtime-docker-cleanup: "Orphan"

    Don't stop the Function's Docker container after rendering.

  render.crossplane.io/runtime-docker-name: "<name>"

    create a container with that name and also reuse it as long as it is running or can be restarted.

  render.crossplane.io/runtime-docker-pull-policy: "Always"

    Always pull the Function's package, even if it already exists locally.
	Other supported values are Never, or IfNotPresent.

Use the standard DOCKER_HOST, DOCKER_API_VERSION, DOCKER_CERT_PATH, and
DOCKER_TLS_VERIFY environment variables to configure how this command connects
to the Docker daemon.`,
		Args: cobra.RangeArgs(2, 3),
		RunE: cmd.run,
	}

	// Flags
	cobraCmd.Flags().StringToStringVar(&cmd.contextFiles, "context-files", nil, "Comma-separated context key-value pairs to pass to the Function pipeline. Values must be files containing JSON.")
	cobraCmd.Flags().StringToStringVar(&cmd.contextValues, "context-values", nil, "Comma-separated context key-value pairs to pass to the Function pipeline. Values must be JSON. Keys take precedence over --context-files.")
	cobraCmd.Flags().BoolVarP(&cmd.includeFunctionResults, "include-function-results", "r", false, "Include informational and warning messages from Functions in the rendered output as resources of kind: Result.")
	cobraCmd.Flags().BoolVarP(&cmd.includeFullXR, "include-full-xr", "x", false, "Include a direct copy of the input XR's spec and metadata fields in the rendered output.")
	cobraCmd.Flags().StringVarP(&cmd.observedResources, "observed-resources", "o", "", "A YAML file or directory of YAML files specifying the observed state of composed resources.")
	cobraCmd.Flags().StringVarP(&cmd.extraResources, "extra-resources", "e", "", "A YAML file or directory of YAML files specifying extra resources to pass to the Function pipeline.")
	cobraCmd.Flags().BoolVarP(&cmd.includeContext, "include-context", "c", false, "Include the context in the rendered output as a resource of kind: Context.")
	cobraCmd.Flags().StringVar(&cmd.functionCredentials, "function-credentials", "", "A YAML file or directory of YAML files specifying credentials to use for Functions to render the XR.")
	cobraCmd.Flags().DurationVar(&cmd.timeout, "timeout", 1*time.Minute, "How long to run before timing out.")
	cobraCmd.Flags().BoolVar(&cmd.refreshCache, "refresh-cache", false, "Force refresh of cached function versions from GitHub")

	return cobraCmd
}

type renderCmd struct {
	// Arguments
	compositeResource string
	composition       string
	functions         string

	// Flags
	contextFiles          map[string]string
	contextValues         map[string]string
	includeFunctionResults bool
	includeFullXR         bool
	observedResources     string
	extraResources        string
	includeContext        bool
	functionCredentials   string
	timeout               time.Duration
	refreshCache          bool

	fs afero.Fs
}

func (c *renderCmd) run(cmd *cobra.Command, args []string) error {
	c.compositeResource = args[0]
	c.composition = args[1]
	if len(args) > 2 {
		c.functions = args[2]
	}

	log := logging.NewNopLogger()

	xr, err := render.LoadCompositeResource(c.fs, c.compositeResource)
	if err != nil {
		return errors.Wrapf(err, "cannot load composite resource from %q", c.compositeResource)
	}

	comp, err := render.LoadComposition(c.fs, c.composition)
	if err != nil {
		return errors.Wrapf(err, "cannot load Composition from %q", c.composition)
	}

	// Validate that Composition's compositeTypeRef matches the XR's GroupVersionKind.
	xrGVK := xr.GetObjectKind().GroupVersionKind()
	compRef := comp.Spec.CompositeTypeRef

	if compRef.Kind != xrGVK.Kind {
		return errors.Errorf("composition's compositeTypeRef.kind (%s) does not match XR's kind (%s)", compRef.Kind, xrGVK.Kind)
	}

	if compRef.APIVersion != xrGVK.GroupVersion().String() {
		return errors.Errorf("composition's compositeTypeRef.apiVersion (%s) does not match XR's apiVersion (%s)", compRef.APIVersion, xrGVK.GroupVersion().String())
	}

	warns, errs := comp.Validate()
	for _, warn := range warns {
		_, _ = fmt.Fprintf(os.Stderr, "WARN(composition): %s\n", warn)
	}
	if len(errs) > 0 {
		return errors.Wrapf(errs.ToAggregate(), "invalid Composition %q", comp.GetName())
	}

	// check if XR's matchLabels have corresponding label at composition
	xrSelector := xr.GetCompositionSelector()
	if xrSelector != nil {
		for key, value := range xrSelector.MatchLabels {
			compValue, exists := comp.Labels[key]
			if !exists {
				return fmt.Errorf("composition %q is missing required label %q", comp.GetName(), key)
			}
			if compValue != value {
				return fmt.Errorf("composition %q has incorrect value for label %q: want %q, got %q",
					comp.GetName(), key, value, compValue)
			}
		}
	}

	if m := comp.Spec.Mode; m == nil || *m != v1.CompositionModePipeline {
		return errors.Errorf("render only supports Composition Function pipelines: Composition %q must use spec.mode: Pipeline", comp.GetName())
	}

	// Load functions - either from file or extract from composition
	var fns []pkgv1.Function
	if c.functions != "" {
		// Load functions from file
		fns, err = render.LoadFunctions(c.fs, c.functions)
		if err != nil {
			return errors.Wrapf(err, "cannot load functions from %q", c.functions)
		}
	} else {
		// Extract functions from composition
		fns, err = ExtractFunctionsFromComposition(comp, c.fs, c.refreshCache)
		if err != nil {
			return errors.Wrapf(err, "cannot extract functions from composition")
		}
		_, _ = fmt.Fprintf(os.Stderr, "INFO: Extracted %d function(s) from composition pipeline\n", len(fns))
		for _, fn := range fns {
			_, _ = fmt.Fprintf(os.Stderr, "INFO: Using function %q with package %q\n", fn.GetName(), fn.Spec.Package)
		}
	}

	fcreds := []corev1.Secret{}
	if c.functionCredentials != "" {
		fcreds, err = render.LoadCredentials(c.fs, c.functionCredentials)
		if err != nil {
			return errors.Wrapf(err, "cannot load secrets from %q", c.functionCredentials)
		}
	}

	ors := []composed.Unstructured{}
	if c.observedResources != "" {
		ors, err = render.LoadObservedResources(c.fs, c.observedResources)
		if err != nil {
			return errors.Wrapf(err, "cannot load observed composed resources from %q", c.observedResources)
		}
	}

	ers := []unstructured.Unstructured{}
	if c.extraResources != "" {
		ers, err = render.LoadExtraResources(c.fs, c.extraResources)
		if err != nil {
			return errors.Wrapf(err, "cannot load extra resources from %q", c.extraResources)
		}
	}

	fctx := map[string][]byte{}
	for k, filename := range c.contextFiles {
		v, err := afero.ReadFile(c.fs, filename)
		if err != nil {
			return errors.Wrapf(err, "cannot read context value for key %q", k)
		}
		fctx[k] = v
	}
	for k, v := range c.contextValues {
		fctx[k] = []byte(v)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	out, err := render.Render(ctx, log, render.Inputs{
		CompositeResource:   xr,
		Composition:         comp,
		Functions:           fns,
		FunctionCredentials: fcreds,
		ObservedResources:   ors,
		ExtraResources:      ers,
		Context:             fctx,
	})
	if err != nil {
		return errors.Wrap(err, "cannot render composite resource")
	}

	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, json.SerializerOptions{Yaml: true})

	if c.includeFullXR {
		xrSpec, err := fieldpath.Pave(xr.Object).GetValue("spec")
		if err != nil {
			return errors.Wrapf(err, "cannot get composite resource spec")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("spec", xrSpec); err != nil {
			return errors.Wrapf(err, "cannot set composite resource spec")
		}

		xrMeta, err := fieldpath.Pave(xr.Object).GetValue("metadata")
		if err != nil {
			return errors.Wrapf(err, "cannot get composite resource metadata")
		}

		if err := fieldpath.Pave(out.CompositeResource.Object).SetValue("metadata", xrMeta); err != nil {
			return errors.Wrapf(err, "cannot set composite resource metadata")
		}
	}

	_, _ = fmt.Fprintln(os.Stdout, "---")
	if err := s.Encode(out.CompositeResource, os.Stdout); err != nil {
		return errors.Wrapf(err, "cannot marshal composite resource %q to YAML", xr.GetName())
	}

	for i := range out.ComposedResources {
		_, _ = fmt.Fprintln(os.Stdout, "---")
		if err := s.Encode(&out.ComposedResources[i], os.Stdout); err != nil {
			return errors.Wrapf(err, "cannot marshal composed resource %q to YAML", out.ComposedResources[i].GetAnnotations()[render.AnnotationKeyCompositionResourceName])
		}
	}

	if c.includeFunctionResults {
		for i := range out.Results {
			_, _ = fmt.Fprintln(os.Stdout, "---")
			if err := s.Encode(&out.Results[i], os.Stdout); err != nil {
				return errors.Wrap(err, "cannot marshal result to YAML")
			}
		}
	}

	if c.includeContext {
		_, _ = fmt.Fprintln(os.Stdout, "---")
		if err := s.Encode(out.Context, os.Stdout); err != nil {
			return errors.Wrap(err, "cannot marshal context to YAML")
		}
	}

	return nil
}

