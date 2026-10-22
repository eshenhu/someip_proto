package semantic

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type analysisState uint8

const (
	analysisUnseen analysisState = iota
	analysisVisiting
	analysisDone
)

type encodedSize struct {
	minimum  uint64
	maximum  uint64
	exact    bool
	overflow bool
}

type payloadInfo struct {
	descriptor  protoreflect.MessageDescriptor
	manifest    *manifestv1alpha1.PayloadTypeManifest
	size        encodedSize
	prefixBytes uint64
	items       []layoutItem
}

type payloadAnalyzer struct {
	normalizer         *normalizer
	states             map[protoreflect.FullName]analysisState
	payloads           map[protoreflect.FullName]*payloadInfo
	enums              map[protoreflect.FullName]*manifestv1alpha1.EnumManifest
	analysisDepth      int
	depthLimitReported map[protoreflect.FullName]bool
}

type layoutItem struct {
	order             uint32
	element           *manifestv1alpha1.PayloadLayoutElement
	size              encodedSize
	member            *manifestv1alpha1.PayloadMemberManifest
	field             protoreflect.FieldDescriptor
	oneof             protoreflect.OneofDescriptor
	headerBytes       uint64
	leadingPadding    uint64
	unionAlternatives []unionAlternativeLayout
}

type unionAlternativeLayout struct {
	field    protoreflect.FieldDescriptor
	manifest *manifestv1alpha1.UnionAlternativeManifest
	size     encodedSize
}

type offsetRange struct {
	minimum uint64
	maximum uint64
}

type alignmentTarget struct {
	member      *manifestv1alpha1.PayloadMemberManifest
	alternative *manifestv1alpha1.UnionAlternativeManifest
}

type alignmentAggregate struct {
	required uint32
	location descriptor.Location
	subject  protoreflect.FullName
	minimum  uint64
	maximum  uint64
	proven   bool
}

type payloadNameHeap []protoreflect.FullName

func (names payloadNameHeap) Len() int {
	return len(names)
}

func (names payloadNameHeap) Less(i, j int) bool {
	return names[i] < names[j]
}

func (names payloadNameHeap) Swap(i, j int) {
	names[i], names[j] = names[j], names[i]
}

func (names *payloadNameHeap) Push(value any) {
	*names = append(*names, value.(protoreflect.FullName))
}

func (names *payloadNameHeap) Pop() any {
	old := *names
	last := len(old) - 1
	value := old[last]
	*names = old[:last]
	return value
}

func (n *normalizer) finalizePayloads(profileFile protoreflect.FileDescriptor) {
	n.finalizePayloadsForFiles([]protoreflect.FileDescriptor{profileFile})
}

func (n *normalizer) finalizePayloadsForFiles(files []protoreflect.FileDescriptor) {
	analyzer := &payloadAnalyzer{
		normalizer:         n,
		states:             make(map[protoreflect.FullName]analysisState),
		payloads:           make(map[protoreflect.FullName]*payloadInfo),
		enums:              make(map[protoreflect.FullName]*manifestv1alpha1.EnumManifest),
		depthLimitReported: make(map[protoreflect.FullName]bool),
	}
	serviceNames := make([]string, 0, len(n.services))
	for name := range n.services {
		serviceNames = append(serviceNames, string(name))
	}
	sort.Strings(serviceNames)
	for _, serviceName := range serviceNames {
		service := n.services[protoreflect.FullName(serviceName)]
		service.maxPayload = 0
		service.memberPayloadBounds = make(map[uint32][]memberDirectionalBound, len(service.manifest.GetMembers()))
		payloadNames := make([]string, 0, len(service.payloads))
		for payloadName := range service.payloads {
			payloadNames = append(payloadNames, string(payloadName))
		}
		sort.Strings(payloadNames)
		for _, payloadName := range payloadNames {
			payload := analyzer.analyzeMessage(service.payloads[protoreflect.FullName(payloadName)])
			if payload.size.maximum > service.maxPayload {
				service.maxPayload = payload.size.maximum
			}
		}
		for _, member := range service.manifest.GetMembers() {
			bounds := analyzer.memberDirectionalPayloadBounds(member)
			service.memberPayloadBounds[member.GetMemberId()] = bounds
			member.DirectionalPayloads = make([]*manifestv1alpha1.MemberDirectionalPayloadManifest, 0, len(bounds))
			for _, bound := range bounds {
				member.DirectionalPayloads = append(member.DirectionalPayloads, &manifestv1alpha1.MemberDirectionalPayloadManifest{
					Direction:   bound.direction,
					EncodedSize: bound.size.manifest(),
				})
			}
		}
	}
	for _, file := range files {
		if file == nil {
			continue
		}
		for index := range file.Messages().Len() {
			analyzer.analyzeAnnotatedMessages(file.Messages().Get(index))
		}
	}
	analyzer.finalizeAlignment()

	payloadNames := make([]string, 0, len(analyzer.payloads))
	for name := range analyzer.payloads {
		payloadNames = append(payloadNames, string(name))
	}
	sort.Strings(payloadNames)
	n.manifest.PayloadTypes = make([]*manifestv1alpha1.PayloadTypeManifest, 0, len(payloadNames))
	for _, name := range payloadNames {
		n.manifest.PayloadTypes = append(n.manifest.PayloadTypes, analyzer.payloads[protoreflect.FullName(name)].manifest)
	}
	enumNames := make([]string, 0, len(analyzer.enums))
	for name := range analyzer.enums {
		enumNames = append(enumNames, string(name))
	}
	sort.Strings(enumNames)
	n.manifest.EnumTypes = make([]*manifestv1alpha1.EnumManifest, 0, len(enumNames))
	for _, name := range enumNames {
		n.manifest.EnumTypes = append(n.manifest.EnumTypes, analyzer.enums[protoreflect.FullName(name)])
	}
}

func (a *payloadAnalyzer) memberDirectionalPayloadBounds(member *manifestv1alpha1.ServiceMemberManifest) []memberDirectionalBound {
	if member == nil {
		return nil
	}
	bounds := make([]memberDirectionalBound, 0, 3)
	appendBound := func(direction manifestv1alpha1.MemberDirection, reference string) {
		bounds = append(bounds, memberDirectionalBound{
			direction: direction,
			size:      a.payloadBound(reference),
		})
	}

	switch member.GetKind() {
	case "event", "field_notifier":
		appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_NOTIFICATION, member.GetResponsePayloadType().GetFqn())
	default:
		appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, member.GetRequestPayloadType().GetFqn())
		method := member.GetMethodOptions()
		if method == nil || method.GetKind() == optionsv1alpha1.MethodKind_METHOD_KIND_FIRE_AND_FORGET {
			break
		}
		switch method.GetResponsePolicy() {
		case optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_RESPONSE_ONLY:
			appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_RESPONSE, member.GetResponsePayloadType().GetFqn())
		case optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_EXCEPTION_ONLY:
			appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_EXCEPTION, method.GetExceptionPayloadType().GetFqn())
		case optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_RESPONSE_OR_EXCEPTION:
			appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_RESPONSE, member.GetResponsePayloadType().GetFqn())
			appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_EXCEPTION, method.GetExceptionPayloadType().GetFqn())
		default:
			// Preserve a conservative result while the method-rule diagnostic
			// explains the invalid authoring choice.
			appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_RESPONSE, member.GetResponsePayloadType().GetFqn())
			if exception := method.GetExceptionPayloadType().GetFqn(); exception != "" {
				appendBound(manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_EXCEPTION, exception)
			}
		}
	}
	sort.Slice(bounds, func(i, j int) bool { return bounds[i].direction < bounds[j].direction })
	return bounds
}

func (a *payloadAnalyzer) payloadBound(reference string) encodedSize {
	if reference == "" || reference == ".google.protobuf.Empty" {
		return fixedSize(0)
	}
	payload := a.payloads[protoreflect.FullName(strings.TrimPrefix(reference, "."))]
	if payload == nil {
		return fixedSize(0)
	}
	return payload.size
}

func (a *payloadAnalyzer) analyzeAnnotatedMessages(message protoreflect.MessageDescriptor) {
	messages := []protoreflect.MessageDescriptor{message}
	for len(messages) > 0 {
		last := len(messages) - 1
		current := messages[last]
		messages = messages[:last]
		if descriptor.MessageOptions(current) != nil {
			a.analyzeMessage(current)
		}
		for index := range current.Messages().Len() {
			messages = append(messages, current.Messages().Get(index))
		}
	}
}

func (a *payloadAnalyzer) maximumPayloadAnalysisDepth() int {
	if a == nil || a.normalizer == nil || a.normalizer.set == nil {
		return descriptor.DefaultLimits().MaxDescriptorDepth
	}
	return a.normalizer.set.Limits().MaxDescriptorDepth
}

func (a *payloadAnalyzer) analyzeMessage(message protoreflect.MessageDescriptor) *payloadInfo {
	if message == nil {
		return &payloadInfo{size: fixedSize(0)}
	}
	a.normalizer.validateAnnotatedOrigin(message, "serializable payload")
	name := message.FullName()
	if payload, found := a.payloads[name]; found {
		if a.states[name] == analysisVisiting {
			a.normalizer.diagnostics.error(
				rulePolicyPayload,
				a.normalizer.set.Locations.Descriptor(message),
				fmt.Sprintf("serializable payload graph is recursive at %s", name),
			)
		}
		return payload
	}
	if a.analysisDepth >= a.maximumPayloadAnalysisDepth() {
		if !a.depthLimitReported[name] {
			a.depthLimitReported[name] = true
			a.normalizer.diagnostics.error(
				rulePolicyPayload,
				a.normalizer.set.Locations.Descriptor(message),
				fmt.Sprintf("serializable payload graph exceeds the %d-level analysis limit at %s", a.maximumPayloadAnalysisDepth(), name),
			)
		}
		return &payloadInfo{descriptor: message, size: fixedSize(0)}
	}
	a.analysisDepth++
	defer func() { a.analysisDepth-- }()

	location := a.normalizer.set.Locations.Descriptor(message)
	payload := &payloadInfo{
		descriptor: message,
		manifest: &manifestv1alpha1.PayloadTypeManifest{
			Type:     cloneReference("." + string(name)),
			Location: location.Manifest(),
		},
		size: fixedSize(0),
	}
	a.payloads[name] = payload
	a.states[name] = analysisVisiting
	defer func() { a.states[name] = analysisDone }()

	if message.ParentFile().Syntax() != protoreflect.Proto3 {
		a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("payload %s must use proto3 syntax", name))
	}
	if message.IsMapEntry() || message.Extensions().Len() > 0 || forbiddenWellKnownType(name) {
		a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("payload %s is not a supported serializable message type", name))
	}

	options := descriptor.MessageOptions(message)
	if options == nil {
		a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("payload %s lacks required message layout metadata", name))
		options = &optionsv1alpha1.MessageOptionsSpec{}
	}
	a.normalizer.validateDocumentationURIs(location, fmt.Sprintf("payload %s", name), options.GetDocumentationUris())
	options = canonicalMessageOptions(options)
	payload.manifest.Options = options
	structLayout := options.GetStructLayout()
	if structLayout == nil {
		a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("payload %s must declare struct layout metadata", name))
		structLayout = &optionsv1alpha1.StructLayout{}
	}
	byteOrder := a.resolveMessageByteOrder(message, options, location)
	lengthWidth := a.resolveStructLengthWidth(structLayout.GetLengthFieldWidth(), location)
	if structLayout.GetExtensible() && lengthWidth == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE {
		a.normalizer.diagnostics.error(ruleStaticEvolution, location, fmt.Sprintf("extensible payload %s requires a nonzero struct length width", name))
	}

	items := a.messageLayoutItems(message, byteOrder)
	sort.Slice(items, func(i, j int) bool { return items[i].order < items[j].order })
	maximumOrder := uint32(0)
	seenOrders := make(map[uint32]struct{}, len(items))
	for expected, item := range items {
		if item.order == 0 || item.order != uint32(expected+1) {
			a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("payload %s layout wire_order values must be contiguous from 1; found %d at position %d", name, item.order, expected+1))
		}
		if _, duplicate := seenOrders[item.order]; duplicate {
			a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("payload %s repeats wire_order %d", name, item.order))
		}
		seenOrders[item.order] = struct{}{}
		if item.order > maximumOrder {
			maximumOrder = item.order
		}
	}

	paddingBefore, endPadding := a.validatePadding(structLayout, maximumOrder, location)
	prefixBytes := widthBytes(lengthWidth)
	body := fixedSize(0)
	for index := range items {
		item := &items[index]
		for _, padding := range paddingBefore[item.order] {
			body = addSize(body, fixedSize(padding))
			item.leadingPadding = saturatingAdd(item.leadingPadding, padding)
		}
		body = addSize(body, item.size)
		payload.manifest.LayoutElements = append(payload.manifest.LayoutElements, item.element)
	}
	for _, padding := range endPadding {
		body = addSize(body, fixedSize(padding))
	}
	if lengthWidth != optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE && body.maximum > maxForWidth(widthBytes(lengthWidth)) {
		a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("payload %s maximum body size %d does not fit its %d-bit struct length prefix", name, body.maximum, widthBytes(lengthWidth)*8))
	}
	payload.size = addSize(fixedSize(prefixBytes), body)
	payload.prefixBytes = prefixBytes
	payload.items = items
	if payload.size.overflow {
		a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("payload %s encoded-size calculation exceeds the supported uint64 bound", name))
	}
	payload.manifest.EncodedSize = payload.size.manifest()
	payload.manifest.EffectiveLayout = &manifestv1alpha1.EffectivePayloadLayout{
		ByteOrder:        byteOrder,
		LengthFieldWidth: lengthWidth,
		Extensible:       structLayout.GetExtensible(),
		MaximumWireOrder: maximumOrder,
	}
	return payload
}

func (a *payloadAnalyzer) messageLayoutItems(message protoreflect.MessageDescriptor, byteOrder optionsv1alpha1.ByteOrder) []layoutItem {
	items := make([]layoutItem, 0, message.Fields().Len()+message.Oneofs().Len())
	for index := range message.Fields().Len() {
		field := message.Fields().Get(index)
		if isRealOneofField(field) {
			continue
		}
		item := a.analyzeOrdinaryField(field, byteOrder)
		items = append(items, item)
	}
	for index := range message.Oneofs().Len() {
		oneof := message.Oneofs().Get(index)
		if oneof.IsSynthetic() {
			continue
		}
		items = append(items, a.analyzeUnion(oneof, byteOrder))
	}
	return items
}

func (a *payloadAnalyzer) analyzeOrdinaryField(field protoreflect.FieldDescriptor, inheritedByteOrder optionsv1alpha1.ByteOrder) layoutItem {
	location := a.normalizer.set.Locations.Descriptor(field)
	options := descriptor.FieldOptions(field)
	if options == nil {
		a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("field %s lacks required layout metadata", field.FullName()))
		options = &optionsv1alpha1.FieldOptionsSpec{}
	}
	a.normalizer.validateDocumentationURIs(location, fmt.Sprintf("field %s", field.FullName()), options.GetDocumentationUris())
	options = canonicalFieldOptions(options)
	if field.IsExtension() || field.IsMap() || field.Kind() == protoreflect.GroupKind {
		a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("field %s uses an unsupported serializable protobuf form", field.FullName()))
	}
	if options.GetUnionTypeId() != 0 || options.GetUnionPaddingBytes() != 0 {
		a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("ordinary field %s cannot declare union-only metadata", field.FullName()))
	}
	if options.GetWireOrder() == 0 {
		a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("ordinary field %s must declare a positive wire_order", field.FullName()))
	}
	a.validateAlignmentRequirement(options, location)

	byteOrder := a.resolveFieldByteOrder(options, inheritedByteOrder, location)
	valueSize, stringFraming := a.analyzeFieldValue(field, options, byteOrder, false)
	size := a.applyCollection(field, options, valueSize, location)
	if size.overflow {
		a.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s encoded-size calculation exceeds the supported uint64 bound", field.FullName()))
	}
	member := &manifestv1alpha1.PayloadMemberManifest{
		Name:        string(field.Name()),
		FieldNumber: uint32(field.Number()),
		Type:        fieldTypeReference(field),
		Options:     options,
		EncodedSize: size.manifest(),
		Location:    location.Manifest(),
		EffectiveLayout: &manifestv1alpha1.EffectiveFieldLayout{
			ByteOrder:         byteOrder,
			LengthFieldWidth:  fieldLengthWidth(field, options),
			StringFraming:     stringFraming,
			UnionPaddingBytes: options.GetUnionPaddingBytes(),
		},
	}
	return layoutItem{
		order:  options.GetWireOrder(),
		size:   size,
		member: member,
		field:  field,
		element: &manifestv1alpha1.PayloadLayoutElement{
			Element: &manifestv1alpha1.PayloadLayoutElement_Member{Member: member},
		},
	}
}

func (a *payloadAnalyzer) analyzeFieldValue(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	byteOrder optionsv1alpha1.ByteOrder,
	inUnion bool,
) (encodedSize, *manifestv1alpha1.EffectiveStringFraming) {
	location := a.normalizer.set.Locations.Descriptor(field)
	a.validateFieldMetadata(field, options, location)
	if explicitPacked(field) {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("serializable repeated numeric field %s cannot use a protobuf packed option", field.FullName()))
	}
	if field.Kind() == protoreflect.StringKind {
		return a.analyzeString(field, options, location)
	}
	if field.Kind() == protoreflect.BytesKind {
		return fixedSize(1), nil
	}
	if field.Kind() == protoreflect.MessageKind {
		if isEmpty(field.Message()) || forbiddenWellKnownType(field.Message().FullName()) {
			a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("message field %s uses an unsupported well-known payload type", field.FullName()))
			return fixedSize(0), nil
		}
		payload := a.analyzeMessage(field.Message())
		if !payload.size.exact && options.GetMaxEncodedBytes() == 0 {
			a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("dynamic struct field %s must declare max_encoded_bytes", field.FullName()))
		}
		if options.GetMaxEncodedBytes() != 0 && options.GetMaxEncodedBytes() < payload.size.maximum {
			a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("field %s max_encoded_bytes %d is below its computed maximum %d", field.FullName(), options.GetMaxEncodedBytes(), payload.size.maximum))
		}
		return payload.size, nil
	}
	if field.Kind() == protoreflect.EnumKind {
		return a.analyzeEnum(field, options, location), nil
	}
	if unsupportedScalarKind(field.Kind()) {
		a.normalizer.diagnostics.error(rulePolicyPayload, location, fmt.Sprintf("field %s uses unsupported protobuf scalar kind %s", field.FullName(), field.Kind()))
		return fixedSize(0), nil
	}
	if inUnion && (field.HasOptionalKeyword() || options.GetOptionalElement()) {
		a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("union alternative %s cannot use optional-element framing", field.FullName()))
	}
	return a.analyzeScalar(field, options, location), nil
}

func (a *payloadAnalyzer) applyCollection(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	elementSize encodedSize,
	location descriptor.Location,
) encodedSize {
	repeated := field.Cardinality() == protoreflect.Repeated || field.Kind() == protoreflect.BytesKind
	optional := options.GetOptionalElement()
	if optional && !field.HasPresence() {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("field %s declares optional_element without protobuf optional presence", field.FullName()))
	}
	if field.HasOptionalKeyword() && !optional {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("protobuf optional field %s must declare optional_element", field.FullName()))
	}
	if optional && repeated {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("field %s cannot combine protobuf optional and array cardinality", field.FullName()))
	}
	if !repeated && !optional {
		if options.GetArray() != nil {
			a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("non-array field %s cannot declare array metadata", field.FullName()))
		}
		return elementSize
	}

	array := options.GetArray()
	if array == nil {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("array or optional field %s must declare array metadata", field.FullName()))
		return elementSize
	}
	resolvedWidth := a.resolveCollectionLengthWidth(array.GetLengthFieldWidth(), location, "array")
	width, valid := arrayWidth(resolvedWidth)
	if !valid {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("field %s declares an invalid array length width", field.FullName()))
		return elementSize
	}
	if optional {
		if width == 0 || array.GetMaxItems() != 1 || array.GetFixedItems() != 0 {
			a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("optional field %s must use a dynamic zero-or-one array", field.FullName()))
		}
		result := prefixedArraySize(width, 0, elementSize.maximum, false)
		if elementSize.maximum > maxForWidth(width) {
			a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("optional field %s maximum element size %d does not fit its %d-bit length prefix", field.FullName(), elementSize.maximum, width*8))
		}
		a.validateCollectionMaximum(field, options, elementSize, result, location)
		return result
	}
	if width == 0 {
		if array.GetFixedItems() == 0 || array.GetMaxItems() != 0 {
			a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("fixed array field %s requires fixed_items and no max_items", field.FullName()))
		}
		result := multiplySize(elementSize, array.GetFixedItems())
		a.validateCollectionMaximum(field, options, elementSize, result, location)
		return result
	}
	if array.GetFixedItems() != 0 || array.GetMaxItems() == 0 {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("dynamic array field %s requires max_items and no fixed_items", field.FullName()))
	}
	maximumBody, maximumOverflow := checkedMultiply(elementSize.maximum, array.GetMaxItems())
	if maximumBody > maxForWidth(width) {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("field %s maximum array body %d does not fit its %d-bit length prefix", field.FullName(), maximumBody, width*8))
	}
	result := prefixedArraySize(width, 0, maximumBody, false)
	result.overflow = result.overflow || elementSize.overflow || maximumOverflow
	a.validateCollectionMaximum(field, options, elementSize, result, location)
	return result
}

func (a *payloadAnalyzer) validateCollectionMaximum(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	elementSize, result encodedSize,
	location descriptor.Location,
) {
	if elementSize.overflow || result.overflow {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("field %s encoded-size calculation exceeds the supported uint64 bound", field.FullName()))
	}
	if (field.Kind() == protoreflect.BytesKind || !elementSize.exact) && options.GetMaxEncodedBytes() == 0 {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("variable-element array field %s must declare max_encoded_bytes", field.FullName()))
	}
	if options.GetMaxEncodedBytes() != 0 && options.GetMaxEncodedBytes() < result.maximum {
		a.normalizer.diagnostics.error(ruleArray, location, fmt.Sprintf("field %s max_encoded_bytes %d is below array encoded maximum %d", field.FullName(), options.GetMaxEncodedBytes(), result.maximum))
	}
}

func (a *payloadAnalyzer) analyzeString(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	location descriptor.Location,
) (encodedSize, *manifestv1alpha1.EffectiveStringFraming) {
	layout := options.GetString_()
	if layout == nil {
		a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("string field %s must declare string metadata", field.FullName()))
		return fixedSize(0), nil
	}
	bom, terminator, validEncoding := stringFraming(layout.GetEncoding())
	if !validEncoding {
		a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("string field %s declares an unsupported encoding", field.FullName()))
	}
	resolvedWidth := a.resolveCollectionLengthWidth(layout.GetLengthFieldWidth(), location, "string")
	width, validWidth := arrayWidth(resolvedWidth)
	if !validWidth {
		a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("string field %s declares an invalid length width", field.FullName()))
	}
	minimumBody := bom + terminator
	framing := &manifestv1alpha1.EffectiveStringFraming{
		BomBytes:            uint32(bom),
		TerminatorBytes:     uint32(terminator),
		FixedLengthZeroFill: width == 0,
	}
	if width == 0 {
		if layout.GetFixedEncodedBytes() == 0 || layout.GetMaxEncodedBytes() != 0 {
			a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("fixed string field %s requires fixed_encoded_bytes and no max_encoded_bytes", field.FullName()))
		}
		if layout.GetFixedEncodedBytes() < minimumBody {
			a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("fixed string field %s is too short for required BOM and terminator", field.FullName()))
		}
		if utf16(layout.GetEncoding()) && layout.GetFixedEncodedBytes()%2 != 0 {
			a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("fixed UTF-16 string field %s must have an even encoded length", field.FullName()))
		}
		return fixedSize(layout.GetFixedEncodedBytes()), framing
	}
	if layout.GetFixedEncodedBytes() != 0 || layout.GetMaxEncodedBytes() == 0 {
		a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("dynamic string field %s requires max_encoded_bytes and no fixed_encoded_bytes", field.FullName()))
	}
	minimumEncoded, minimumOverflow := checkedAdd(width, minimumBody)
	if layout.GetMaxEncodedBytes() < minimumEncoded {
		a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("dynamic string field %s maximum is too short for required BOM and terminator", field.FullName()))
	}
	bodyMaximum := uint64(0)
	if layout.GetMaxEncodedBytes() >= width {
		bodyMaximum = layout.GetMaxEncodedBytes() - width
	}
	if utf16(layout.GetEncoding()) && bodyMaximum%2 != 0 {
		a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("dynamic UTF-16 string field %s must have an even body encoded length", field.FullName()))
	}
	if bodyMaximum > maxForWidth(width) {
		a.normalizer.diagnostics.error(ruleString, location, fmt.Sprintf("dynamic string field %s maximum does not fit its length prefix", field.FullName()))
	}
	return encodedSize{minimum: minimumEncoded, maximum: layout.GetMaxEncodedBytes(), exact: false, overflow: minimumOverflow}, framing
}

func (a *payloadAnalyzer) analyzeScalar(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	location descriptor.Location,
) encodedSize {
	expected, valid := validScalarRepresentations(field.Kind())
	if !valid || !expected[options.GetScalarRepresentation()] {
		a.normalizer.diagnostics.error(ruleType, location, fmt.Sprintf("field %s has incompatible SOME/IP scalar representation %s", field.FullName(), options.GetScalarRepresentation()))
	}
	if options.GetBitfield() != nil && field.Kind() == protoreflect.Uint32Kind {
		a.validateBitfield(field, options, location)
	}
	return fixedSize(scalarBytes(options.GetScalarRepresentation()))
}

func (a *payloadAnalyzer) analyzeEnum(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	location descriptor.Location,
) encodedSize {
	if options.GetScalarRepresentation() != optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_ENUM_8 &&
		options.GetScalarRepresentation() != optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_ENUM_16 &&
		options.GetScalarRepresentation() != optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_ENUM_32 {
		a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("enum field %s must declare an enum scalar representation", field.FullName()))
	}
	width := uint32(scalarBytes(options.GetScalarRepresentation()) * 8)
	enumOptions := descriptor.EnumOptions(field.Enum())
	if enumOptions == nil {
		a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("enum %s lacks required enum metadata", field.Enum().FullName()))
	} else {
		if enumOptions.GetWireWidthBits() != width {
			a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("enum %s wire width does not match field %s scalar representation", field.Enum().FullName(), field.FullName()))
		}
		if enumOptions.GetWireWidthBits() != 8 && enumOptions.GetWireWidthBits() != 16 && enumOptions.GetWireWidthBits() != 32 {
			a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("enum %s must use an 8-, 16-, or 32-bit wire width", field.Enum().FullName()))
		}
	}
	hasZero := false
	for index := range field.Enum().Values().Len() {
		value := field.Enum().Values().Get(index)
		if value.Number() < 0 || value.Number() > 0x7fffffff {
			a.normalizer.diagnostics.error(ruleEnum, a.normalizer.set.Locations.Descriptor(value), fmt.Sprintf("enum value %s is outside the supported nonnegative range", value.FullName()))
		}
		if value.Number() == 0 {
			hasZero = true
		}
		if maximum := maximumEnumValue(width); maximum >= 0 && int64(value.Number()) > maximum {
			a.normalizer.diagnostics.error(ruleEnum, a.normalizer.set.Locations.Descriptor(value), fmt.Sprintf("enum value %s does not fit the declared %d-bit wire representation", value.FullName(), width))
		}
		valueOptions := descriptor.EnumValueOptions(value)
		if valueOptions != nil && valueOptions.GetNonSendablePlaceholder() {
			if value.Number() != 0 || !strings.HasSuffix(string(value.Name()), "_UNSPECIFIED") {
				a.normalizer.diagnostics.error(ruleEnum, a.normalizer.set.Locations.Descriptor(value), fmt.Sprintf("non-sendable enum placeholder %s must be named *_UNSPECIFIED with numeric value zero", value.FullName()))
			}
		} else if value.Number() == 0 && strings.HasSuffix(string(value.Name()), "_UNSPECIFIED") {
			a.normalizer.diagnostics.error(ruleEnum, a.normalizer.set.Locations.Descriptor(value), fmt.Sprintf("enum placeholder %s must be marked non-sendable", value.FullName()))
		}
	}
	if !hasZero {
		a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("enum %s must declare a numeric-zero value", field.Enum().FullName()))
	}
	a.recordEnum(field.Enum(), enumOptions)
	return fixedSize(scalarBytes(options.GetScalarRepresentation()))
}

func maximumEnumValue(width uint32) int64 {
	switch width {
	case 8:
		return math.MaxUint8
	case 16:
		return math.MaxUint16
	case 32:
		return 0x7fffffff
	default:
		return -1
	}
}

func (a *payloadAnalyzer) recordEnum(enum protoreflect.EnumDescriptor, options *optionsv1alpha1.EnumOptionsSpec) {
	if enum == nil {
		return
	}
	a.normalizer.validateAnnotatedOrigin(enum, "serializable enum")
	if _, found := a.enums[enum.FullName()]; found {
		return
	}
	a.normalizer.validateDocumentationURIs(a.normalizer.set.Locations.Descriptor(enum), fmt.Sprintf("enum %s", enum.FullName()), options.GetDocumentationUris())
	options = canonicalEnumOptions(options)
	manifest := &manifestv1alpha1.EnumManifest{
		Type:     cloneReference("." + string(enum.FullName())),
		Options:  options,
		Location: a.normalizer.set.Locations.Descriptor(enum).Manifest(),
	}
	a.enums[enum.FullName()] = manifest
	for index := range enum.Values().Len() {
		value := enum.Values().Get(index)
		valueOptions := descriptor.EnumValueOptions(value)
		a.normalizer.validateDocumentationURIs(a.normalizer.set.Locations.Descriptor(value), fmt.Sprintf("enum value %s", value.FullName()), valueOptions.GetDocumentationUris())
		manifest.Values = append(manifest.Values, &manifestv1alpha1.EnumValueManifest{
			Name:     string(value.Name()),
			Number:   int32(value.Number()),
			Options:  canonicalEnumValueOptions(valueOptions),
			Location: a.normalizer.set.Locations.Descriptor(value).Manifest(),
		})
	}
	sort.Slice(manifest.Values, func(i, j int) bool {
		if manifest.Values[i].GetNumber() != manifest.Values[j].GetNumber() {
			return manifest.Values[i].GetNumber() < manifest.Values[j].GetNumber()
		}
		return manifest.Values[i].GetName() < manifest.Values[j].GetName()
	})
}

func (a *payloadAnalyzer) validateBitfield(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	location descriptor.Location,
) {
	bitfield := options.GetBitfield()
	if field.Kind() != protoreflect.Uint32Kind || (bitfield.GetWidthBits() != 8 && bitfield.GetWidthBits() != 16 && bitfield.GetWidthBits() != 32) {
		a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("bitfield %s must use uint32 with an 8-, 16-, or 32-bit width", field.FullName()))
	}
	if scalarBytes(options.GetScalarRepresentation())*8 != uint64(bitfield.GetWidthBits()) {
		a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("bitfield %s width does not match scalar representation", field.FullName()))
	}
	seen := make(map[uint32]struct{})
	names := make(map[string]struct{})
	for _, namedBit := range bitfield.GetNamedBits() {
		if namedBit.GetName() == "" || namedBit.GetBitIndex() >= bitfield.GetWidthBits() {
			a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("bitfield %s contains an invalid named bit", field.FullName()))
		}
		if _, found := seen[namedBit.GetBitIndex()]; found {
			a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("bitfield %s repeats bit index %d", field.FullName(), namedBit.GetBitIndex()))
		}
		seen[namedBit.GetBitIndex()] = struct{}{}
		if _, found := names[namedBit.GetName()]; found {
			a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("bitfield %s repeats bit name %q", field.FullName(), namedBit.GetName()))
		}
		names[namedBit.GetName()] = struct{}{}
	}
	if bitfield.GetClosed() && (bitfield.GetWidthBits() == 8 || bitfield.GetWidthBits() == 16 || bitfield.GetWidthBits() == 32) && len(seen) != int(bitfield.GetWidthBits()) {
		a.normalizer.diagnostics.error(ruleEnum, location, fmt.Sprintf("closed bitfield %s must name every bit in its %d-bit representation", field.FullName(), bitfield.GetWidthBits()))
	}
}

func (a *payloadAnalyzer) analyzeUnion(oneof protoreflect.OneofDescriptor, inheritedByteOrder optionsv1alpha1.ByteOrder) layoutItem {
	location := a.normalizer.set.Locations.Descriptor(oneof)
	options := descriptor.OneofOptions(oneof)
	if options == nil {
		a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("oneof %s lacks required union metadata", oneof.FullName()))
		options = &optionsv1alpha1.OneofOptionsSpec{}
	}
	a.normalizer.validateDocumentationURIs(location, fmt.Sprintf("union %s", oneof.FullName()), options.GetDocumentationUris())
	options = canonicalOneofOptions(options)
	if options.GetByteOrder() != optionsv1alpha1.ByteOrder_BYTE_ORDER_UNSPECIFIED {
		a.normalizer.diagnostics.error(ruleLayout, location, fmt.Sprintf("union %s cannot override network byte order for its headers", oneof.FullName()))
	}
	if options.GetWireOrder() == 0 {
		a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("union %s must declare a positive wire_order", oneof.FullName()))
	}
	effectiveOptions := effectiveUnionOptions(options)
	discriminatorBytes := discriminatorBytes(effectiveOptions.GetDiscriminatorWidthBits())
	if discriminatorBytes == 0 {
		discriminatorBytes = 4
		if effectiveOptions.GetDiscriminatorWidthBits() != 0 {
			a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("union %s discriminator width must be 8, 16, or 32 bits", oneof.FullName()))
		}
	}
	lengthWidth := a.resolveUnionLengthWidth(effectiveOptions.GetLengthFieldWidth(), location)
	headerOrder := effectiveOptions.GetHeaderOrder()
	if headerOrder != optionsv1alpha1.UnionHeaderOrder_UNION_HEADER_ORDER_TYPE_THEN_LENGTH &&
		headerOrder != optionsv1alpha1.UnionHeaderOrder_UNION_HEADER_ORDER_LENGTH_THEN_TYPE {
		a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("union %s has an invalid header order", oneof.FullName()))
		headerOrder = optionsv1alpha1.UnionHeaderOrder_UNION_HEADER_ORDER_LENGTH_THEN_TYPE
	}

	union := &manifestv1alpha1.UnionManifest{
		Options:         options,
		HeaderByteOrder: optionsv1alpha1.ByteOrder_BYTE_ORDER_BIG_ENDIAN,
		Location:        location.Manifest(),
		EffectiveLayout: &manifestv1alpha1.EffectiveUnionLayout{
			HeaderOrder:            headerOrder,
			DiscriminatorWidthBits: uint32(discriminatorBytes * 8),
			LengthFieldWidth:       lengthWidth,
			AllowsNull:             effectiveOptions.GetAllowsNull(),
			NullTypeId:             0,
		},
	}
	seenIDs := make(map[uint32]struct{})
	declaredIDs := make([]unionTypeID, 0, oneof.Fields().Len())
	alternativeLayouts := make([]unionAlternativeLayout, 0, oneof.Fields().Len())
	maximumSlot := uint64(0)
	minimumSlot := uint64(math.MaxUint64)
	commonExact := true
	firstExactSize := uint64(0)
	for index := range oneof.Fields().Len() {
		field := oneof.Fields().Get(index)
		fieldLocation := a.normalizer.set.Locations.Descriptor(field)
		fieldOptions := descriptor.FieldOptions(field)
		if fieldOptions == nil {
			a.normalizer.diagnostics.error(ruleUnion, fieldLocation, fmt.Sprintf("union alternative %s lacks required field metadata", field.FullName()))
			fieldOptions = &optionsv1alpha1.FieldOptionsSpec{}
		}
		a.normalizer.validateDocumentationURIs(fieldLocation, fmt.Sprintf("union alternative %s", field.FullName()), fieldOptions.GetDocumentationUris())
		fieldOptions = canonicalFieldOptions(fieldOptions)
		if fieldOptions.GetWireOrder() != 0 {
			a.normalizer.diagnostics.error(ruleUnion, fieldLocation, fmt.Sprintf("union alternative %s must not declare ordinary wire_order", field.FullName()))
		}
		a.validateAlignmentRequirement(fieldOptions, fieldLocation)
		if _, found := seenIDs[fieldOptions.GetUnionTypeId()]; found {
			a.normalizer.diagnostics.error(ruleUnion, fieldLocation, fmt.Sprintf("union %s repeats type ID %d", oneof.FullName(), fieldOptions.GetUnionTypeId()))
		}
		seenIDs[fieldOptions.GetUnionTypeId()] = struct{}{}
		declaredIDs = append(declaredIDs, unionTypeID{
			identifier: fieldOptions.GetUnionTypeId(),
			location:   fieldLocation,
			name:       field.FullName(),
		})
		fieldByteOrder := a.resolveFieldByteOrder(fieldOptions, inheritedByteOrder, fieldLocation)
		valueSize, stringFraming := a.analyzeFieldValue(field, fieldOptions, fieldByteOrder, true)
		valueSize = a.applyUnionCollection(field, fieldOptions, valueSize, fieldLocation)
		valueSize = addSize(valueSize, fixedSize(fieldOptions.GetUnionPaddingBytes()))
		if valueSize.overflow {
			a.normalizer.diagnostics.error(ruleUnion, fieldLocation, fmt.Sprintf("union alternative %s encoded-size calculation exceeds the supported uint64 bound", field.FullName()))
		}
		if !valueSize.exact && fieldOptions.GetMaxEncodedBytes() == 0 {
			a.normalizer.diagnostics.error(ruleUnion, fieldLocation, fmt.Sprintf("dynamic union alternative %s must declare max_encoded_bytes", field.FullName()))
		}
		if fieldOptions.GetMaxEncodedBytes() != 0 && fieldOptions.GetMaxEncodedBytes() < valueSize.maximum {
			a.normalizer.diagnostics.error(ruleUnion, fieldLocation, fmt.Sprintf("union alternative %s max_encoded_bytes is below computed maximum", field.FullName()))
		}
		alternative := &manifestv1alpha1.UnionAlternativeManifest{
			Name:                 string(field.Name()),
			FieldNumber:          uint32(field.Number()),
			Type:                 fieldTypeReference(field),
			Options:              fieldOptions,
			TypeId:               fieldOptions.GetUnionTypeId(),
			DeclaredPaddingBytes: fieldOptions.GetUnionPaddingBytes(),
			EncodedSize:          valueSize.manifest(),
			Location:             fieldLocation.Manifest(),
			EffectiveLayout: &manifestv1alpha1.EffectiveFieldLayout{
				ByteOrder:         fieldByteOrder,
				LengthFieldWidth:  fieldLengthWidth(field, fieldOptions),
				StringFraming:     stringFraming,
				UnionPaddingBytes: fieldOptions.GetUnionPaddingBytes(),
			},
		}
		union.Alternatives = append(union.Alternatives, alternative)
		alternativeLayouts = append(alternativeLayouts, unionAlternativeLayout{
			field:    field,
			manifest: alternative,
			size:     valueSize,
		})
		if valueSize.maximum > maximumSlot {
			maximumSlot = valueSize.maximum
		}
		if valueSize.minimum < minimumSlot {
			minimumSlot = valueSize.minimum
		}
		if index == 0 {
			firstExactSize = valueSize.maximum
		} else if !valueSize.exact || valueSize.maximum != firstExactSize {
			commonExact = false
		}
		if !valueSize.exact {
			commonExact = false
		}
	}
	if oneof.Fields().Len() == 0 {
		a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("union %s must declare at least one alternative", oneof.FullName()))
		minimumSlot = 0
	}
	if effectiveOptions.GetAllowsNull() {
		nullAlternative := &manifestv1alpha1.UnionAlternativeManifest{
			Name:        "NULL",
			TypeId:      0,
			EncodedSize: fixedSize(0).manifest(),
			Location:    location.Manifest(),
		}
		if lengthWidth == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE {
			nullAlternative.FixedDerivedZeroFillBytes = maximumSlot
		}
		union.Alternatives = append(union.Alternatives, nullAlternative)
		minimumSlot = 0
		if maximumSlot != 0 {
			commonExact = false
		}
	}
	sort.Slice(declaredIDs, func(i, j int) bool { return declaredIDs[i].identifier < declaredIDs[j].identifier })
	for index, declared := range declaredIDs {
		if declared.identifier != uint32(index+1) {
			a.normalizer.diagnostics.error(ruleUnion, declared.location, fmt.Sprintf("union alternative %s must use a contiguous type ID; found %d, want %d", declared.name, declared.identifier, index+1))
		}
	}
	sort.Slice(union.Alternatives, func(i, j int) bool { return union.Alternatives[i].GetTypeId() < union.Alternatives[j].GetTypeId() })
	union.EffectiveValueSlotBytes = maximumSlot
	headerBytes := discriminatorBytes + widthBytes(lengthWidth)
	trailingPaddingBytes := options.GetPaddingBytes()
	var unionSize encodedSize
	if lengthWidth == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE {
		if !commonExact {
			a.normalizer.diagnostics.warning(ruleUnionZeroWidth, location, fmt.Sprintf("zero-width union %s requires derived zero-fill to its %d-byte effective value slot", oneof.FullName(), maximumSlot))
		}
		for _, alternative := range union.Alternatives {
			if alternative.GetEncodedSize().GetExactBytes() != 0 || alternative.GetEncodedSize().GetMinimumBytes() == alternative.GetEncodedSize().GetMaximumBytes() {
				alternative.FixedDerivedZeroFillBytes = maximumSlot - alternative.GetEncodedSize().GetMaximumBytes()
			} else {
				alternative.PadToEffectiveSlot = true
			}
		}
		unionSize = addSize(addSize(fixedSize(headerBytes), fixedSize(maximumSlot)), fixedSize(trailingPaddingBytes))
	} else {
		if maximumSlot > maxForWidth(widthBytes(lengthWidth)) {
			a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("union %s value extent %d does not fit its length prefix", oneof.FullName(), maximumSlot))
		}
		unionSize = addSize(addSize(fixedSize(headerBytes), encodedSize{
			minimum: minimumSlot,
			maximum: maximumSlot,
			exact:   commonExact && minimumSlot == maximumSlot,
		}), fixedSize(trailingPaddingBytes))
	}
	if unionSize.overflow {
		a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("union %s encoded-size calculation exceeds the supported uint64 bound", oneof.FullName()))
	}
	union.EncodedSize = unionSize.manifest()
	computedMaximum := union.GetEncodedSize().GetMaximumBytes()
	if options.GetMaxEncodedBytes() == 0 || options.GetMaxEncodedBytes() < computedMaximum {
		a.normalizer.diagnostics.error(ruleUnion, location, fmt.Sprintf("union %s max_encoded_bytes must bound its computed maximum %d", oneof.FullName(), computedMaximum))
	}
	return layoutItem{
		order:             options.GetWireOrder(),
		size:              unionSize,
		oneof:             oneof,
		headerBytes:       headerBytes,
		unionAlternatives: alternativeLayouts,
		element: &manifestv1alpha1.PayloadLayoutElement{
			Element: &manifestv1alpha1.PayloadLayoutElement_Union{Union: union},
		},
	}
}

func effectiveUnionOptions(options *optionsv1alpha1.OneofOptionsSpec) *optionsv1alpha1.OneofOptionsSpec {
	effective := proto.Clone(options).(*optionsv1alpha1.OneofOptionsSpec)
	if effective.GetDiscriminatorWidthBits() == 0 {
		effective.DiscriminatorWidthBits = 32
	}
	if effective.GetLengthFieldWidth() == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_UNSPECIFIED {
		effective.LengthFieldWidth = optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32
	}
	if effective.GetHeaderOrder() == optionsv1alpha1.UnionHeaderOrder_UNION_HEADER_ORDER_UNSPECIFIED {
		effective.HeaderOrder = optionsv1alpha1.UnionHeaderOrder_UNION_HEADER_ORDER_LENGTH_THEN_TYPE
	}
	return effective
}

type unionTypeID struct {
	identifier uint32
	location   descriptor.Location
	name       protoreflect.FullName
}

func (a *payloadAnalyzer) applyUnionCollection(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	value encodedSize,
	location descriptor.Location,
) encodedSize {
	if field.HasOptionalKeyword() {
		return value
	}
	if field.Cardinality() != protoreflect.Repeated && field.Kind() != protoreflect.BytesKind {
		return value
	}
	return a.applyCollection(field, options, value, location)
}

func (a *payloadAnalyzer) validatePadding(
	layout *optionsv1alpha1.StructLayout,
	maximumOrder uint32,
	location descriptor.Location,
) (map[uint32][]uint64, []uint64) {
	before := make(map[uint32][]uint64)
	var atEnd []uint64
	for _, padding := range layout.GetPadding() {
		if padding.GetByteCount() == 0 {
			a.normalizer.diagnostics.error(ruleLayoutFraming, location, "explicit padding byte_count must be positive")
		}
		switch position := padding.GetPosition().(type) {
		case *optionsv1alpha1.Padding_BeforeWireOrder:
			if position.BeforeWireOrder == 0 || position.BeforeWireOrder > maximumOrder {
				a.normalizer.diagnostics.error(ruleLayoutFraming, location, "padding before_wire_order must reference an existing layout element")
			}
			before[position.BeforeWireOrder] = append(before[position.BeforeWireOrder], padding.GetByteCount())
		case *optionsv1alpha1.Padding_AtEnd:
			if !position.AtEnd {
				a.normalizer.diagnostics.error(ruleLayoutFraming, location, "at_end padding position must be true")
			}
			atEnd = append(atEnd, padding.GetByteCount())
		default:
			a.normalizer.diagnostics.error(ruleLayoutFraming, location, "padding must declare before_wire_order or at_end position")
		}
	}
	return before, atEnd
}

func (a *payloadAnalyzer) validateAlignmentRequirement(options *optionsv1alpha1.FieldOptionsSpec, location descriptor.Location) {
	required := options.GetAlignmentBytes()
	if required == 0 || validAlignmentRequirement(required) {
		return
	}
	a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("alignment %d must be one of 1, 2, 4, or 8", required))
}

func (a *payloadAnalyzer) finalizeAlignment() {
	cyclic := a.cyclicPayloads()
	order := a.alignmentOrder(cyclic)
	contexts := make(map[protoreflect.FullName]offsetRange, len(a.payloads))
	for _, name := range order {
		payload := a.payloads[name]
		start := saturatingAdd(16, payload.prefixBytes)
		contexts[name] = offsetRange{minimum: start, maximum: start}
	}

	members := make(map[*manifestv1alpha1.PayloadMemberManifest]*alignmentAggregate)
	alternatives := make(map[*manifestv1alpha1.UnionAlternativeManifest]*alignmentAggregate)
	for _, name := range order {
		payload := a.payloads[name]
		context := contexts[name]
		cursor := context
		for _, item := range payload.items {
			cursor = addOffset(cursor, item.leadingPadding)
			if item.member != nil {
				a.recordAlignment(members, alignmentTarget{member: item.member}, item.field, cursor)
				a.mergeNestedPayloadContext(item.field, item.member.GetOptions(), cursor, contexts, cyclic)
			}
			if len(item.unionAlternatives) > 0 {
				alternativeStart := addOffset(cursor, item.headerBytes)
				for _, alternative := range item.unionAlternatives {
					a.recordAlternativeAlignment(alternatives, alignmentTarget{alternative: alternative.manifest}, alternative.field, alternativeStart)
					a.mergeNestedPayloadContext(alternative.field, alternative.manifest.GetOptions(), alternativeStart, contexts, cyclic)
				}
			}
			cursor = addOffsetSize(cursor, item.size)
		}
	}
	a.publishAlignment(members, alternatives)
}

func (a *payloadAnalyzer) alignmentOrder(cyclic map[protoreflect.FullName]bool) []protoreflect.FullName {
	dependents := make(map[protoreflect.FullName][]protoreflect.FullName, len(a.payloads))
	inDegree := make(map[protoreflect.FullName]uint32, len(a.payloads))
	for _, name := range a.payloadNames() {
		if cyclic[name] {
			continue
		}
		inDegree[name] = 0
	}
	for parent := range inDegree {
		seen := make(map[protoreflect.FullName]struct{})
		for _, child := range a.payloadDependencies(a.payloads[parent]) {
			if cyclic[child] || child == parent {
				continue
			}
			if _, found := inDegree[child]; !found {
				continue
			}
			if _, duplicate := seen[child]; duplicate {
				continue
			}
			seen[child] = struct{}{}
			dependents[parent] = append(dependents[parent], child)
			inDegree[child]++
		}
	}
	ready := make(payloadNameHeap, 0, len(inDegree))
	for name, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, name)
		}
	}
	heap.Init(&ready)
	order := make([]protoreflect.FullName, 0, len(inDegree))
	for ready.Len() > 0 {
		name := heap.Pop(&ready).(protoreflect.FullName)
		order = append(order, name)
		for _, child := range dependents[name] {
			inDegree[child]--
			if inDegree[child] == 0 {
				heap.Push(&ready, child)
			}
		}
	}
	return order
}

func (a *payloadAnalyzer) payloadNames() []protoreflect.FullName {
	names := make([]protoreflect.FullName, 0, len(a.payloads))
	for name := range a.payloads {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}

func (a *payloadAnalyzer) mergeNestedPayloadContext(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	start offsetRange,
	contexts map[protoreflect.FullName]offsetRange,
	cyclic map[protoreflect.FullName]bool,
) {
	if field == nil || field.Kind() != protoreflect.MessageKind || isEmpty(field.Message()) {
		return
	}
	name := field.Message().FullName()
	payload, found := a.payloads[name]
	if !found || cyclic[name] {
		return
	}
	nested, present := nestedBodyOffsetRange(field, options, start, payload)
	if !present {
		return
	}
	current, found := contexts[name]
	if !found {
		return
	}
	if mergeOffsetRange(&current, nested) {
		contexts[name] = current
	}
}

func nestedBodyOffsetRange(
	field protoreflect.FieldDescriptor,
	options *optionsv1alpha1.FieldOptionsSpec,
	start offsetRange,
	payload *payloadInfo,
) (offsetRange, bool) {
	if field == nil || options == nil || payload == nil {
		return offsetRange{}, false
	}
	prefixBytes := uint64(0)
	maximumItems := uint64(1)
	isCollection := field.Cardinality() == protoreflect.Repeated || options.GetOptionalElement()
	if isCollection {
		array := options.GetArray()
		if array == nil {
			return offsetRange{}, false
		}
		prefixBytes = widthBytes(effectiveCollectionLengthWidth(array.GetLengthFieldWidth()))
		switch {
		case options.GetOptionalElement():
			maximumItems = 1
		case prefixBytes == 0:
			maximumItems = array.GetFixedItems()
		default:
			maximumItems = array.GetMaxItems()
		}
		if maximumItems == 0 {
			return offsetRange{}, false
		}
	}
	return offsetRange{
		minimum: saturatingAdd(saturatingAdd(start.minimum, prefixBytes), payload.prefixBytes),
		maximum: saturatingAdd(saturatingAdd(saturatingAdd(start.maximum, prefixBytes), saturatingMultiply(maximumItems-1, payload.size.maximum)), payload.prefixBytes),
	}, true
}

func (a *payloadAnalyzer) recordAlignment(
	aggregates map[*manifestv1alpha1.PayloadMemberManifest]*alignmentAggregate,
	target alignmentTarget,
	field protoreflect.FieldDescriptor,
	offset offsetRange,
) {
	if target.member == nil || field == nil {
		return
	}
	required := target.member.GetOptions().GetAlignmentBytes()
	if required == 0 || !validAlignmentRequirement(required) {
		return
	}
	aggregate, found := aggregates[target.member]
	if !found {
		aggregate = &alignmentAggregate{
			required: required,
			location: a.normalizer.set.Locations.Descriptor(field),
			subject:  field.FullName(),
			minimum:  offset.minimum,
			maximum:  offset.maximum,
			proven:   alignmentProven(required, offset),
		}
		aggregates[target.member] = aggregate
		return
	}
	aggregate.minimum = minUint(aggregate.minimum, offset.minimum)
	aggregate.maximum = maxUint(aggregate.maximum, offset.maximum)
	aggregate.proven = aggregate.proven && alignmentProven(required, offset)
}

func (a *payloadAnalyzer) recordAlternativeAlignment(
	aggregates map[*manifestv1alpha1.UnionAlternativeManifest]*alignmentAggregate,
	target alignmentTarget,
	field protoreflect.FieldDescriptor,
	offset offsetRange,
) {
	if target.alternative == nil || field == nil {
		return
	}
	required := target.alternative.GetOptions().GetAlignmentBytes()
	if required == 0 || !validAlignmentRequirement(required) {
		return
	}
	aggregate, found := aggregates[target.alternative]
	if !found {
		aggregate = &alignmentAggregate{
			required: required,
			location: a.normalizer.set.Locations.Descriptor(field),
			subject:  field.FullName(),
			minimum:  offset.minimum,
			maximum:  offset.maximum,
			proven:   alignmentProven(required, offset),
		}
		aggregates[target.alternative] = aggregate
		return
	}
	aggregate.minimum = minUint(aggregate.minimum, offset.minimum)
	aggregate.maximum = maxUint(aggregate.maximum, offset.maximum)
	aggregate.proven = aggregate.proven && alignmentProven(required, offset)
}

func (a *payloadAnalyzer) publishAlignment(
	members map[*manifestv1alpha1.PayloadMemberManifest]*alignmentAggregate,
	alternatives map[*manifestv1alpha1.UnionAlternativeManifest]*alignmentAggregate,
) {
	for member, aggregate := range members {
		member.Alignment = a.alignmentManifest(aggregate)
	}
	for alternative, aggregate := range alternatives {
		alternative.Alignment = a.alignmentManifest(aggregate)
	}
}

func (a *payloadAnalyzer) alignmentManifest(aggregate *alignmentAggregate) *manifestv1alpha1.AlignmentAnalysis {
	analysis := &manifestv1alpha1.AlignmentAnalysis{
		RequiredAlignmentBytes: aggregate.required,
		Proven:                 aggregate.proven,
		MinimumOffsetBytes:     aggregate.minimum,
		MaximumOffsetBytes:     aggregate.maximum,
	}
	if !aggregate.proven {
		a.normalizer.diagnostics.warning(ruleAlignment, aggregate.location, fmt.Sprintf("alignment of %d bytes for %s is not provable for offset range %d..%d", aggregate.required, aggregate.subject, aggregate.minimum, aggregate.maximum))
	}
	return analysis
}

func (a *payloadAnalyzer) cyclicPayloads() map[protoreflect.FullName]bool {
	const (
		unseen uint8 = iota
		visiting
		done
	)
	type traversalFrame struct {
		name         protoreflect.FullName
		dependencies []protoreflect.FullName
		next         int
	}
	state := make(map[protoreflect.FullName]uint8, len(a.payloads))
	cyclic := make(map[protoreflect.FullName]bool)
	for _, root := range a.payloadNames() {
		if state[root] != unseen {
			continue
		}
		state[root] = visiting
		frames := []traversalFrame{{name: root, dependencies: a.payloadDependencies(a.payloads[root])}}
		active := []protoreflect.FullName{root}
		positions := map[protoreflect.FullName]int{root: 0}
		for len(frames) > 0 {
			frame := &frames[len(frames)-1]
			if frame.next == len(frame.dependencies) {
				state[frame.name] = done
				delete(positions, frame.name)
				frames = frames[:len(frames)-1]
				active = active[:len(active)-1]
				continue
			}
			dependency := frame.dependencies[frame.next]
			frame.next++
			if _, found := a.payloads[dependency]; !found {
				continue
			}
			switch state[dependency] {
			case unseen:
				state[dependency] = visiting
				positions[dependency] = len(active)
				active = append(active, dependency)
				frames = append(frames, traversalFrame{name: dependency, dependencies: a.payloadDependencies(a.payloads[dependency])})
			case visiting:
				for _, name := range active[positions[dependency]:] {
					cyclic[name] = true
				}
			}
		}
	}
	return cyclic
}

func (a *payloadAnalyzer) payloadDependencies(payload *payloadInfo) []protoreflect.FullName {
	if payload == nil {
		return nil
	}
	dependencies := make([]protoreflect.FullName, 0)
	for _, item := range payload.items {
		if item.field != nil && item.field.Kind() == protoreflect.MessageKind {
			dependencies = append(dependencies, item.field.Message().FullName())
		}
		for _, alternative := range item.unionAlternatives {
			if alternative.field.Kind() == protoreflect.MessageKind {
				dependencies = append(dependencies, alternative.field.Message().FullName())
			}
		}
	}
	return dependencies
}

func addOffset(offset offsetRange, bytes uint64) offsetRange {
	return offsetRange{
		minimum: saturatingAdd(offset.minimum, bytes),
		maximum: saturatingAdd(offset.maximum, bytes),
	}
}

func addOffsetSize(offset offsetRange, size encodedSize) offsetRange {
	return offsetRange{
		minimum: saturatingAdd(offset.minimum, size.minimum),
		maximum: saturatingAdd(offset.maximum, size.maximum),
	}
}

func mergeOffsetRange(current *offsetRange, update offsetRange) bool {
	minimum := minUint(current.minimum, update.minimum)
	maximum := maxUint(current.maximum, update.maximum)
	if minimum == current.minimum && maximum == current.maximum {
		return false
	}
	current.minimum = minimum
	current.maximum = maximum
	return true
}

func alignmentProven(required uint32, offset offsetRange) bool {
	return required == 1 || (offset.minimum == offset.maximum && offset.minimum%uint64(required) == 0)
}

func validAlignmentRequirement(required uint32) bool {
	return required == 1 || required == 2 || required == 4 || required == 8
}

func minUint(left, right uint64) uint64 {
	if left < right {
		return left
	}
	return right
}

func maxUint(left, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}

func (a *payloadAnalyzer) resolveMessageByteOrder(
	message protoreflect.MessageDescriptor,
	options *optionsv1alpha1.MessageOptionsSpec,
	location descriptor.Location,
) optionsv1alpha1.ByteOrder {
	fileOptions := descriptor.FileOptions(message.ParentFile())
	byteOrder := optionsv1alpha1.ByteOrder_BYTE_ORDER_UNSPECIFIED
	if fileOptions != nil {
		byteOrder = fileOptions.GetDefaultByteOrder()
	}
	if options.GetByteOrder() != optionsv1alpha1.ByteOrder_BYTE_ORDER_UNSPECIFIED {
		byteOrder = options.GetByteOrder()
	}
	if !validByteOrder(byteOrder) {
		a.normalizer.diagnostics.error(ruleLayout, location, fmt.Sprintf("payload %s does not resolve a valid byte order", message.FullName()))
		return optionsv1alpha1.ByteOrder_BYTE_ORDER_BIG_ENDIAN
	}
	return byteOrder
}

func (a *payloadAnalyzer) resolveFieldByteOrder(
	options *optionsv1alpha1.FieldOptionsSpec,
	inherited optionsv1alpha1.ByteOrder,
	location descriptor.Location,
) optionsv1alpha1.ByteOrder {
	byteOrder := inherited
	if options.GetByteOrder() != optionsv1alpha1.ByteOrder_BYTE_ORDER_UNSPECIFIED {
		byteOrder = options.GetByteOrder()
	}
	if !validByteOrder(byteOrder) {
		a.normalizer.diagnostics.error(ruleLayout, location, "field does not resolve a valid byte order")
		return optionsv1alpha1.ByteOrder_BYTE_ORDER_BIG_ENDIAN
	}
	return byteOrder
}

func (a *payloadAnalyzer) resolveStructLengthWidth(width optionsv1alpha1.LengthFieldWidth, location descriptor.Location) optionsv1alpha1.LengthFieldWidth {
	if width == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_UNSPECIFIED {
		return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE
	}
	if _, valid := arrayWidth(width); !valid {
		a.normalizer.diagnostics.error(ruleLayoutFraming, location, "struct length width must be NONE, 8, 16, or 32")
		return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE
	}
	return width
}

func (a *payloadAnalyzer) resolveUnionLengthWidth(width optionsv1alpha1.LengthFieldWidth, location descriptor.Location) optionsv1alpha1.LengthFieldWidth {
	if width == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_UNSPECIFIED {
		return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32
	}
	if _, valid := arrayWidth(width); !valid {
		a.normalizer.diagnostics.error(ruleUnion, location, "union length width must be NONE, 8, 16, or 32")
		return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32
	}
	return width
}

func (a *payloadAnalyzer) resolveCollectionLengthWidth(
	width optionsv1alpha1.LengthFieldWidth,
	location descriptor.Location,
	element string,
) optionsv1alpha1.LengthFieldWidth {
	if width == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_UNSPECIFIED {
		return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32
	}
	if _, valid := arrayWidth(width); !valid {
		a.normalizer.diagnostics.error(ruleLayoutFraming, location, fmt.Sprintf("%s length width must be NONE, 8, 16, or 32", element))
		return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32
	}
	return width
}

func fixedSize(bytes uint64) encodedSize {
	return encodedSize{minimum: bytes, maximum: bytes, exact: true}
}

func addSize(left, right encodedSize) encodedSize {
	minimum, minimumOverflow := checkedAdd(left.minimum, right.minimum)
	maximum, maximumOverflow := checkedAdd(left.maximum, right.maximum)
	return encodedSize{
		minimum:  minimum,
		maximum:  maximum,
		exact:    left.exact && right.exact && !minimumOverflow && !maximumOverflow,
		overflow: left.overflow || right.overflow || minimumOverflow || maximumOverflow,
	}
}

func multiplySize(size encodedSize, count uint64) encodedSize {
	minimum, minimumOverflow := checkedMultiply(size.minimum, count)
	maximum, maximumOverflow := checkedMultiply(size.maximum, count)
	return encodedSize{
		minimum:  minimum,
		maximum:  maximum,
		exact:    size.exact && !minimumOverflow && !maximumOverflow,
		overflow: size.overflow || minimumOverflow || maximumOverflow,
	}
}

func saturatingMultiply(left, right uint64) uint64 {
	value, _ := checkedMultiply(left, right)
	return value
}

func saturatingAdd(left, right uint64) uint64 {
	value, _ := checkedAdd(left, right)
	return value
}

func prefixedArraySize(width, minimumBody, maximumBody uint64, exact bool) encodedSize {
	prefix := width
	minimum, minimumOverflow := checkedAdd(prefix, minimumBody)
	maximum, maximumOverflow := checkedAdd(prefix, maximumBody)
	return encodedSize{
		minimum:  minimum,
		maximum:  maximum,
		exact:    exact && minimumBody == maximumBody && !minimumOverflow && !maximumOverflow,
		overflow: minimumOverflow || maximumOverflow,
	}
}

func checkedAdd(left, right uint64) (uint64, bool) {
	if math.MaxUint64-left < right {
		return math.MaxUint64, true
	}
	return left + right, false
}

func checkedMultiply(left, right uint64) (uint64, bool) {
	if left != 0 && right > math.MaxUint64/left {
		return math.MaxUint64, true
	}
	return left * right, false
}

func (s encodedSize) manifest() *manifestv1alpha1.EncodedSize {
	exact := uint64(0)
	if s.exact {
		exact = s.maximum
	}
	return &manifestv1alpha1.EncodedSize{
		MinimumBytes: s.minimum,
		ExactBytes:   exact,
		MaximumBytes: s.maximum,
	}
}

func encodedSizeFromManifest(size *manifestv1alpha1.EncodedSize) encodedSize {
	if size == nil {
		return fixedSize(0)
	}
	return encodedSize{
		minimum: size.GetMinimumBytes(),
		maximum: size.GetMaximumBytes(),
		exact:   size.GetExactBytes() != 0 || (size.GetMinimumBytes() == 0 && size.GetMaximumBytes() == 0),
	}
}

func arrayWidth(width optionsv1alpha1.LengthFieldWidth) (uint64, bool) {
	switch width {
	case optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE:
		return 0, true
	case optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_8:
		return 1, true
	case optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_16:
		return 2, true
	case optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32:
		return 4, true
	default:
		return 0, false
	}
}

func widthBytes(width optionsv1alpha1.LengthFieldWidth) uint64 {
	bytes, _ := arrayWidth(width)
	return bytes
}

func maxForWidth(bytes uint64) uint64 {
	switch bytes {
	case 1:
		return math.MaxUint8
	case 2:
		return math.MaxUint16
	case 4:
		return math.MaxUint32
	default:
		return math.MaxUint64
	}
}

func discriminatorBytes(width uint32) uint64 {
	switch width {
	case 8:
		return 1
	case 16:
		return 2
	case 32:
		return 4
	default:
		return 0
	}
}

func scalarBytes(representation optionsv1alpha1.ScalarRepresentation) uint64 {
	switch representation {
	case optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_BOOLEAN_8,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_8,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_8,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_ENUM_8:
		return 1
	case optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_16,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_16,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_ENUM_16:
		return 2
	case optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_32,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_32,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_FLOAT_32,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_ENUM_32:
		return 4
	case optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_64,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_64,
		optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_FLOAT_64:
		return 8
	default:
		return 0
	}
}

func validScalarRepresentations(kind protoreflect.Kind) (map[optionsv1alpha1.ScalarRepresentation]bool, bool) {
	representations := make(map[optionsv1alpha1.ScalarRepresentation]bool)
	switch kind {
	case protoreflect.BoolKind:
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_BOOLEAN_8] = true
	case protoreflect.Int32Kind:
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_8] = true
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_16] = true
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_32] = true
	case protoreflect.Int64Kind:
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_SIGNED_INTEGER_64] = true
	case protoreflect.Uint32Kind:
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_8] = true
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_16] = true
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_32] = true
	case protoreflect.Uint64Kind:
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_64] = true
	case protoreflect.FloatKind:
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_FLOAT_32] = true
	case protoreflect.DoubleKind:
		representations[optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_FLOAT_64] = true
	default:
		return nil, false
	}
	return representations, true
}

func unsupportedScalarKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return true
	default:
		return false
	}
}

func explicitPacked(field protoreflect.FieldDescriptor) bool {
	if field.Cardinality() != protoreflect.Repeated || !field.Kind().IsValid() {
		return false
	}
	options, _ := field.Options().(*descriptorpb.FieldOptions)
	return options != nil && options.Packed != nil
}

func stringFraming(encoding optionsv1alpha1.StringEncoding) (uint64, uint64, bool) {
	switch encoding {
	case optionsv1alpha1.StringEncoding_STRING_ENCODING_ASCII:
		return 0, 1, true
	case optionsv1alpha1.StringEncoding_STRING_ENCODING_UTF8:
		return 3, 1, true
	case optionsv1alpha1.StringEncoding_STRING_ENCODING_UTF16_BE,
		optionsv1alpha1.StringEncoding_STRING_ENCODING_UTF16_LE:
		return 2, 2, true
	default:
		return 0, 0, false
	}
}

func utf16(encoding optionsv1alpha1.StringEncoding) bool {
	return encoding == optionsv1alpha1.StringEncoding_STRING_ENCODING_UTF16_BE || encoding == optionsv1alpha1.StringEncoding_STRING_ENCODING_UTF16_LE
}

func validByteOrder(byteOrder optionsv1alpha1.ByteOrder) bool {
	return byteOrder == optionsv1alpha1.ByteOrder_BYTE_ORDER_BIG_ENDIAN || byteOrder == optionsv1alpha1.ByteOrder_BYTE_ORDER_LITTLE_ENDIAN
}

func fieldTypeReference(field protoreflect.FieldDescriptor) *optionsv1alpha1.ProtobufReference {
	switch field.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return cloneReference("." + string(field.Message().FullName()))
	case protoreflect.EnumKind:
		return cloneReference("." + string(field.Enum().FullName()))
	default:
		return nil
	}
}

func fieldLengthWidth(field protoreflect.FieldDescriptor, options *optionsv1alpha1.FieldOptionsSpec) optionsv1alpha1.LengthFieldWidth {
	if field.Kind() == protoreflect.StringKind && options.GetString_() != nil {
		return effectiveCollectionLengthWidth(options.GetString_().GetLengthFieldWidth())
	}
	if options.GetArray() != nil {
		return effectiveCollectionLengthWidth(options.GetArray().GetLengthFieldWidth())
	}
	return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_UNSPECIFIED
}

func effectiveCollectionLengthWidth(width optionsv1alpha1.LengthFieldWidth) optionsv1alpha1.LengthFieldWidth {
	if width == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_UNSPECIFIED {
		return optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32
	}
	return width
}

func isRealOneofField(field protoreflect.FieldDescriptor) bool {
	return field.ContainingOneof() != nil && !field.ContainingOneof().IsSynthetic()
}

func forbiddenWellKnownType(name protoreflect.FullName) bool {
	return strings.HasPrefix(string(name), "google.protobuf.")
}
