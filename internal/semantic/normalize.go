package semantic

import (
	"fmt"
	"sort"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Profiles returns all declared deployment profiles in stable name and source
// order. The returned definitions are read-only views of the descriptor set.
func Profiles(set *descriptor.Set) []string {
	definitions := discoverProfiles(set)
	profiles := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		profiles = append(profiles, definition.definition.GetProfileName())
	}
	return profiles
}

// Normalize validates one selected deployment profile and builds its canonical
// manifest. Semantic errors remain in Result.Diagnostics and block artifact
// writers, while warnings still permit output.
func Normalize(set *descriptor.Set, profileName string, config Config) *Result {
	if config.ToolVersion == "" {
		config.ToolVersion = DefaultConfig().ToolVersion
	}
	normalizer := &normalizer{
		set:                       set,
		config:                    config,
		services:                  make(map[protoreflect.FullName]*serviceInfo),
		reportedUnapprovedOrigins: make(map[string]struct{}),
		manifest: &manifestv1alpha1.Manifest{
			Provenance: &manifestv1alpha1.ManifestProvenance{
				ManifestFormatVersion: 1,
				OptionsApiVersion:     "v1alpha1",
				InputFdsSha256:        set.InputSHA256,
				ToolVersion:           config.ToolVersion,
				SourceRevision:        config.SourceRevision,
				NormativeSourceCommit: normativeSourceCommit,
			},
			UnknownOptions: set.UnknownOptions,
		},
	}
	normalizer.validateArtifactProvenance()
	normalizer.validateAllFileDocumentationURIs()

	definitions := discoverProfiles(set)
	normalizer.validateUniqueProfileNames(definitions)
	matching := make([]profileDefinition, 0, 1)
	for _, definition := range definitions {
		if definition.definition.GetProfileName() == profileName {
			matching = append(matching, definition)
		}
	}
	if profileName == "" {
		normalizer.diagnostics.error(ruleProfile, descriptor.Location{}, "a profile name is required")
	} else if len(matching) == 0 {
		normalizer.diagnostics.error(ruleProfile, descriptor.Location{}, fmt.Sprintf("deployment profile %q was not found", profileName))
	} else if len(matching) == 1 {
		normalizer.normalizeProfile(matching[0])
	}
	normalizer.validateUnboundAnnotatedServices()

	for _, unknown := range normalizer.manifest.UnknownOptions {
		normalizer.diagnostics.warning(
			ruleArtifact,
			locationFromManifest(unknown.GetLocation()),
			fmt.Sprintf("preserved unknown option field %d on %s", unknown.GetFieldNumber(), unknown.GetExtendeeKind().String()),
		)
	}

	normalizer.manifest.Diagnostics = normalizer.diagnostics.sorted()
	return &Result{
		Manifest:    normalizer.manifest,
		Diagnostics: normalizer.manifest.Diagnostics,
	}
}

func (n *normalizer) validateAllFileDocumentationURIs() {
	for _, file := range n.set.AllFiles() {
		options := descriptor.FileOptions(file)
		if options == nil {
			continue
		}
		n.validateDocumentationURIs(
			n.set.Locations.FileOptions(string(file.Path())),
			fmt.Sprintf("file %q", file.Path()),
			options.GetDocumentationUris(),
		)
	}
}

func (n *normalizer) validateUniqueProfileNames(definitions []profileDefinition) {
	byName := make(map[string][]profileDefinition, len(definitions))
	for _, definition := range definitions {
		byName[definition.definition.GetProfileName()] = append(byName[definition.definition.GetProfileName()], definition)
	}
	for name, matching := range byName {
		if len(matching) < 2 {
			continue
		}
		for _, definition := range matching {
			n.diagnostics.error(
				ruleProfile,
				n.fileOptionsLocation(definition),
				fmt.Sprintf("deployment profile name %q is declared more than once", name),
			)
		}
	}
}

func (n *normalizer) validateUnboundAnnotatedServices() {
	selected := make(map[protoreflect.FullName]struct{}, len(n.services))
	for name := range n.services {
		selected[name] = struct{}{}
	}
	audit := &normalizer{
		set:                       n.set,
		config:                    n.config,
		services:                  make(map[protoreflect.FullName]*serviceInfo),
		manifest:                  &manifestv1alpha1.Manifest{},
		reportedUnapprovedOrigins: n.reportedUnapprovedOrigins,
	}
	for _, file := range n.set.AllFiles() {
		for index := range file.Services().Len() {
			service := file.Services().Get(index)
			if descriptor.ServiceOptions(service) == nil {
				continue
			}
			if _, found := selected[service.FullName()]; found {
				continue
			}
			audit.normalizeService(service)
		}
	}
	audit.finalizePayloadsForFiles(nil)
	n.diagnostics.diagnostics = append(n.diagnostics.diagnostics, audit.diagnostics.diagnostics...)
}

// ValidateAll validates every deployment profile declared by an FDS. It is the
// backend used by the CLI validate command when no --profile is selected.
func ValidateAll(set *descriptor.Set, config Config) []*Result {
	definitions := discoverProfiles(set)
	if len(definitions) == 0 {
		return []*Result{Normalize(set, "", config)}
	}
	results := make([]*Result, 0, len(definitions))
	for _, definition := range definitions {
		results = append(results, Normalize(set, definition.definition.GetProfileName(), config))
	}
	return results
}

func discoverProfiles(set *descriptor.Set) []profileDefinition {
	definitions := make([]profileDefinition, 0)
	for _, file := range set.AllFiles() {
		options := descriptor.FileOptions(file)
		if options.GetDeploymentProfile() == nil {
			continue
		}
		definitions = append(definitions, profileDefinition{
			file:       file,
			definition: options.GetDeploymentProfile(),
		})
	}
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].definition.GetProfileName() != definitions[j].definition.GetProfileName() {
			return definitions[i].definition.GetProfileName() < definitions[j].definition.GetProfileName()
		}
		return definitions[i].file.Path() < definitions[j].file.Path()
	})
	return definitions
}

func (n *normalizer) fileOptionsLocation(definition profileDefinition) descriptor.Location {
	return n.set.Locations.FileOptions(string(definition.file.Path()))
}

func locationFromManifest(location *manifestv1alpha1.SourceLocation) descriptor.Location {
	if location == nil {
		return descriptor.Location{}
	}
	return descriptor.Location{
		ImportPath: location.GetImportPath(),
		Line:       location.GetLine(),
		Column:     location.GetColumn(),
	}
}

func cloneReference(name string) *optionsv1alpha1.ProtobufReference {
	return &optionsv1alpha1.ProtobufReference{Fqn: name}
}
