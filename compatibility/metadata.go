package compatibility

import (
	"sort"
	"strconv"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type descriptorMetadata struct {
	comments      string
	deprecated    bool
	sourcePresent bool
}

func metadataPatchChanged(
	baselineSet, currentSet *descriptor.Set,
	baseline, current *manifestv1alpha1.Manifest,
) bool {
	baselineMetadata := descriptorMetadataIndex(baselineSet)
	currentMetadata := descriptorMetadataIndex(currentSet)
	keys := stableMetadataKeys(baseline, current, baselineMetadata, currentMetadata)
	for key := range keys {
		baselineValue, baselineFound := baselineMetadata[key]
		currentValue, currentFound := currentMetadata[key]
		if baselineFound != currentFound || baselineValue != currentValue {
			return true
		}
	}
	if fileDocumentationURIChanged(keys, baselineSet, currentSet) {
		return true
	}
	return documentationURIChanged(baseline, current) || returnCodeDescriptionChanged(baseline, current)
}

func metadataComparisonReliable(
	baselineSet, currentSet *descriptor.Set,
	baseline, current *manifestv1alpha1.Manifest,
) bool {
	if baselineSet != nil && currentSet != nil && baselineSet.InputSHA256 != "" && baselineSet.InputSHA256 == currentSet.InputSHA256 {
		return true
	}
	baselineMetadata := descriptorMetadataIndex(baselineSet)
	currentMetadata := descriptorMetadataIndex(currentSet)
	keys := stableMetadataKeys(baseline, current, baselineMetadata, currentMetadata)
	for key := range keys {
		baselineValue, baselineFound := baselineMetadata[key]
		currentValue, currentFound := currentMetadata[key]
		if !baselineFound || !currentFound || !baselineValue.sourcePresent || !currentValue.sourcePresent {
			return false
		}
	}
	return true
}

func fileDocumentationURIChanged(keys map[string]struct{}, baseline, current *descriptor.Set) bool {
	baselineURIs := fileDocumentationURIs(baseline)
	currentURIs := fileDocumentationURIs(current)
	for key := range keys {
		if !strings.HasPrefix(key, "file:") {
			continue
		}
		if !sameStringSet(baselineURIs[key], currentURIs[key]) {
			return true
		}
	}
	return false
}

func fileDocumentationURIs(set *descriptor.Set) map[string][]string {
	uris := make(map[string][]string)
	if set == nil || set.Raw == nil {
		return uris
	}
	for _, file := range set.Raw.GetFile() {
		if file.GetOptions() == nil {
			continue
		}
		options, ok := proto.GetExtension(file.GetOptions(), optionsv1alpha1.E_File).(*optionsv1alpha1.FileOptionsSpec)
		if !ok || options == nil {
			continue
		}
		uris["file:"+file.GetName()] = append([]string(nil), options.GetDocumentationUris()...)
	}
	return uris
}

func documentationURIChanged(baseline, current *manifestv1alpha1.Manifest) bool {
	if !sameStringSet(baseline.GetProfile().GetDocumentationUris(), current.GetProfile().GetDocumentationUris()) {
		return true
	}
	baselineServices := servicesByKey(baseline.GetServices())
	currentServices := servicesByKey(current.GetServices())
	for key, baselineService := range baselineServices {
		currentService, found := currentServices[key]
		if !found || baselineService.GetService().GetFqn() != currentService.GetService().GetFqn() {
			continue
		}
		if !sameStringSet(baselineService.GetDocumentationUris(), currentService.GetDocumentationUris()) {
			return true
		}
		baselineMembers := membersByID(baselineService.GetMembers())
		currentMembers := membersByID(currentService.GetMembers())
		for identifier, baselineMember := range baselineMembers {
			currentMember, found := currentMembers[identifier]
			if !found || !sameMemberContract(baselineMember, currentMember) {
				continue
			}
			if !sameStringSet(baselineMember.GetMethodOptions().GetDocumentationUris(), currentMember.GetMethodOptions().GetDocumentationUris()) {
				return true
			}
		}
	}

	baselinePayloads := payloadsByName(baseline.GetPayloadTypes())
	currentPayloads := payloadsByName(current.GetPayloadTypes())
	for name, baselinePayload := range baselinePayloads {
		currentPayload, found := currentPayloads[name]
		if found && proto.Equal(payloadContract(baselinePayload), payloadContract(currentPayload)) && !samePayloadDocumentationURIs(baselinePayload, currentPayload) {
			return true
		}
	}
	baselineEnums := enumTypesByName(baseline.GetEnumTypes())
	currentEnums := enumTypesByName(current.GetEnumTypes())
	for name, baselineEnum := range baselineEnums {
		currentEnum, found := currentEnums[name]
		if found && proto.Equal(enumContract(baselineEnum), enumContract(currentEnum)) && !sameEnumDocumentationURIs(baselineEnum, currentEnum) {
			return true
		}
	}
	return false
}

func samePayloadDocumentationURIs(left, right *manifestv1alpha1.PayloadTypeManifest) bool {
	if !sameStringSet(left.GetOptions().GetDocumentationUris(), right.GetOptions().GetDocumentationUris()) || len(left.GetLayoutElements()) != len(right.GetLayoutElements()) {
		return false
	}
	for index, leftElement := range left.GetLayoutElements() {
		rightElement := right.GetLayoutElements()[index]
		if leftMember, rightMember := leftElement.GetMember(), rightElement.GetMember(); leftMember != nil || rightMember != nil {
			if leftMember == nil || rightMember == nil || !sameStringSet(leftMember.GetOptions().GetDocumentationUris(), rightMember.GetOptions().GetDocumentationUris()) {
				return false
			}
			continue
		}
		leftUnion, rightUnion := leftElement.GetUnion(), rightElement.GetUnion()
		if leftUnion == nil || rightUnion == nil || !sameStringSet(leftUnion.GetOptions().GetDocumentationUris(), rightUnion.GetOptions().GetDocumentationUris()) || len(leftUnion.GetAlternatives()) != len(rightUnion.GetAlternatives()) {
			return false
		}
		for alternativeIndex, leftAlternative := range leftUnion.GetAlternatives() {
			rightAlternative := rightUnion.GetAlternatives()[alternativeIndex]
			if !sameStringSet(leftAlternative.GetOptions().GetDocumentationUris(), rightAlternative.GetOptions().GetDocumentationUris()) {
				return false
			}
		}
	}
	return true
}

func sameEnumDocumentationURIs(left, right *manifestv1alpha1.EnumManifest) bool {
	if !sameStringSet(left.GetOptions().GetDocumentationUris(), right.GetOptions().GetDocumentationUris()) || len(left.GetValues()) != len(right.GetValues()) {
		return false
	}
	for index, leftValue := range left.GetValues() {
		rightValue := right.GetValues()[index]
		if !sameStringSet(leftValue.GetOptions().GetDocumentationUris(), rightValue.GetOptions().GetDocumentationUris()) {
			return false
		}
	}
	return true
}

func stableMetadataKeys(
	baseline, current *manifestv1alpha1.Manifest,
	baselineMetadata, currentMetadata map[string]descriptorMetadata,
) map[string]struct{} {
	keys := make(map[string]struct{})
	baselineServices := servicesByKey(baseline.GetServices())
	currentServices := servicesByKey(current.GetServices())
	baselinePayloads := payloadsByName(baseline.GetPayloadTypes())
	currentPayloads := payloadsByName(current.GetPayloadTypes())
	baselineEnums := enumTypesByName(baseline.GetEnumTypes())
	currentEnums := enumTypesByName(current.GetEnumTypes())
	addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(baseline.GetProfile()))
	addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(current.GetProfile()))
	for key, baselineService := range baselineServices {
		currentService, found := currentServices[key]
		if !found || baselineService.GetService().GetFqn() != currentService.GetService().GetFqn() {
			continue
		}
		serviceName := strings.TrimPrefix(baselineService.GetService().GetFqn(), ".")
		keys["service:"+serviceName] = struct{}{}
		addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(baselineService))
		addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(currentService))
		for _, member := range baselineService.GetMembers() {
			if member.GetMethodOptions() == nil {
				continue
			}
			keys["method:"+serviceName+"."+member.GetName()] = struct{}{}
		}
		for payloadName := range reachablePayloads(baselineService, baselinePayloads) {
			baselinePayload := baselinePayloads[payloadName]
			currentPayload, found := currentPayloads[payloadName]
			if !found || !proto.Equal(payloadContract(baselinePayload), payloadContract(currentPayload)) {
				continue
			}
			addMetadataKeysForDescriptorName(keys, baselineMetadata, currentMetadata, strings.TrimPrefix(payloadName, "."))
			addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(baselinePayload))
			addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(currentPayload))
		}
		for enumName := range reachableEnumTypes(baselineService, baselinePayloads, baselineEnums) {
			baselineEnum := baselineEnums[enumName]
			currentEnum, found := currentEnums[enumName]
			if !found || !proto.Equal(enumContract(baselineEnum), enumContract(currentEnum)) {
				continue
			}
			addMetadataKeysForDescriptorName(keys, baselineMetadata, currentMetadata, strings.TrimPrefix(enumName, "."))
			addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(baselineEnum))
			addFileMetadataKey(keys, baselineMetadata, currentMetadata, sourceLocation(currentEnum))
		}
	}
	return keys
}

func addFileMetadataKey(
	keys map[string]struct{},
	baseline, current map[string]descriptorMetadata,
	location *manifestv1alpha1.SourceLocation,
) {
	if location == nil || location.GetImportPath() == "" {
		return
	}
	key := "file:" + location.GetImportPath()
	if _, found := baseline[key]; found {
		keys[key] = struct{}{}
		return
	}
	if _, found := current[key]; found {
		keys[key] = struct{}{}
	}
}

func addMetadataKeysForDescriptorName(
	keys map[string]struct{},
	baseline, current map[string]descriptorMetadata,
	name string,
) {
	for key := range baseline {
		if metadataKeyMatchesName(key, name) {
			keys[key] = struct{}{}
		}
	}
	for key := range current {
		if metadataKeyMatchesName(key, name) {
			keys[key] = struct{}{}
		}
	}
}

func metadataKeyMatchesName(key, name string) bool {
	separator := strings.IndexByte(key, ':')
	if separator < 0 {
		return false
	}
	descriptorName := key[separator+1:]
	return descriptorName == name || strings.HasPrefix(descriptorName, name+".")
}

func returnCodeDescriptionChanged(baseline, current *manifestv1alpha1.Manifest) bool {
	baselineServices := servicesByKey(baseline.GetServices())
	currentServices := servicesByKey(current.GetServices())
	for key, baselineService := range baselineServices {
		currentService, found := currentServices[key]
		if !found || baselineService.GetService().GetFqn() != currentService.GetService().GetFqn() {
			continue
		}
		currentMembers := membersByID(currentService.GetMembers())
		for _, baselineMember := range baselineService.GetMembers() {
			currentMember, found := currentMembers[baselineMember.GetMemberId()]
			if !found || !sameMemberContract(baselineMember, currentMember) {
				continue
			}
			if !sameReturnCodeDescriptions(baselineMember.GetMethodOptions().GetReturnCodes(), currentMember.GetMethodOptions().GetReturnCodes()) {
				return true
			}
		}
	}
	return false
}

func sameReturnCodeDescriptions(left, right []*optionsv1alpha1.ReturnCode) bool {
	if len(left) != len(right) {
		return false
	}
	descriptions := make(map[uint32]string, len(left))
	for _, code := range left {
		descriptions[code.GetCode()] = code.GetDescription()
	}
	for _, code := range right {
		if descriptions[code.GetCode()] != code.GetDescription() {
			return false
		}
	}
	return true
}

func descriptorMetadataIndex(set *descriptor.Set) map[string]descriptorMetadata {
	metadata := make(map[string]descriptorMetadata)
	if set == nil || set.Raw == nil {
		return metadata
	}
	for _, file := range set.Raw.GetFile() {
		comments := sourceCommentsByPath(file)
		packageName := file.GetPackage()
		metadata["file:"+file.GetName()] = metadataForSourcePath(comments, nil, false)
		for index, message := range file.GetMessageType() {
			collectMessageMetadata(metadata, comments, message, []int32{4, int32(index)}, qualifiedDescriptorName(packageName, message.GetName()))
		}
		for index, enum := range file.GetEnumType() {
			collectEnumMetadata(metadata, comments, enum, []int32{5, int32(index)}, qualifiedDescriptorName(packageName, enum.GetName()))
		}
		for index, service := range file.GetService() {
			serviceName := qualifiedDescriptorName(packageName, service.GetName())
			servicePath := []int32{6, int32(index)}
			metadata["service:"+serviceName] = metadataForSourcePath(comments, servicePath, service.GetOptions().GetDeprecated())
			for methodIndex, method := range service.GetMethod() {
				methodPath := appendDescriptorPath(servicePath, 2, int32(methodIndex))
				metadata["method:"+qualifiedDescriptorName(serviceName, method.GetName())] = metadataForSourcePath(comments, methodPath, method.GetOptions().GetDeprecated())
			}
		}
	}
	return metadata
}

func metadataForSourcePath(comments map[string]string, path []int32, deprecated bool) descriptorMetadata {
	comment, sourcePresent := comments[sourcePathKey(path)]
	return descriptorMetadata{
		comments:      comment,
		deprecated:    deprecated,
		sourcePresent: sourcePresent,
	}
}

func collectMessageMetadata(
	metadata map[string]descriptorMetadata,
	comments map[string]string,
	message *descriptorpb.DescriptorProto,
	path []int32,
	name string,
) {
	metadata["message:"+name] = metadataForSourcePath(comments, path, message.GetOptions().GetDeprecated())
	for index, field := range message.GetField() {
		fieldPath := appendDescriptorPath(path, 2, int32(index))
		metadata["field:"+qualifiedDescriptorName(name, field.GetName())] = metadataForSourcePath(comments, fieldPath, field.GetOptions().GetDeprecated())
	}
	for index, oneof := range message.GetOneofDecl() {
		oneofPath := appendDescriptorPath(path, 8, int32(index))
		metadata["oneof:"+qualifiedDescriptorName(name, oneof.GetName())] = metadataForSourcePath(comments, oneofPath, false)
	}
	for index, nested := range message.GetNestedType() {
		collectMessageMetadata(metadata, comments, nested, appendDescriptorPath(path, 3, int32(index)), qualifiedDescriptorName(name, nested.GetName()))
	}
	for index, enum := range message.GetEnumType() {
		collectEnumMetadata(metadata, comments, enum, appendDescriptorPath(path, 4, int32(index)), qualifiedDescriptorName(name, enum.GetName()))
	}
}

func collectEnumMetadata(
	metadata map[string]descriptorMetadata,
	comments map[string]string,
	enum *descriptorpb.EnumDescriptorProto,
	path []int32,
	name string,
) {
	metadata["enum:"+name] = metadataForSourcePath(comments, path, enum.GetOptions().GetDeprecated())
	for index, value := range enum.GetValue() {
		valuePath := appendDescriptorPath(path, 2, int32(index))
		metadata["enum_value:"+qualifiedDescriptorName(name, value.GetName())] = metadataForSourcePath(comments, valuePath, value.GetOptions().GetDeprecated())
	}
}

func sourceCommentsByPath(file *descriptorpb.FileDescriptorProto) map[string]string {
	byPath := make(map[string][]string, len(file.GetSourceCodeInfo().GetLocation()))
	for _, location := range file.GetSourceCodeInfo().GetLocation() {
		parts := make([]string, 0, len(location.GetLeadingDetachedComments())+2)
		parts = append(parts, location.GetLeadingDetachedComments()...)
		parts = append(parts, location.GetLeadingComments(), location.GetTrailingComments())
		byPath[sourcePathKey(location.GetPath())] = append(byPath[sourcePathKey(location.GetPath())], strings.Join(parts, "\x00"))
	}
	comments := make(map[string]string, len(byPath))
	for path, values := range byPath {
		sort.Strings(values)
		comments[path] = strings.Join(values, "\x00")
	}
	return comments
}

func sourcePathKey(path []int32) string {
	if len(path) == 0 {
		return ""
	}
	parts := make([]string, len(path))
	for index, value := range path {
		parts[index] = strconv.FormatInt(int64(value), 10)
	}
	return strings.Join(parts, "/")
}

func appendDescriptorPath(path []int32, values ...int32) []int32 {
	result := make([]int32, 0, len(path)+len(values))
	result = append(result, path...)
	return append(result, values...)
}

func qualifiedDescriptorName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
