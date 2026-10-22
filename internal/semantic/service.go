package semantic

import (
	"fmt"
	"sort"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (n *normalizer) normalizeService(service protoreflect.ServiceDescriptor) *serviceInfo {
	if normalized, found := n.services[service.FullName()]; found {
		return normalized
	}
	location := n.set.Locations.Descriptor(service)
	options := descriptor.ServiceOptions(service)
	if options == nil {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("referenced service %s has no project service option", service.FullName()))
		return nil
	}
	n.validateAnnotatedOrigin(service, "annotated service")
	if options.GetServiceLineage().GetId() == "" {
		n.diagnostics.error(rulePolicyLineage, location, fmt.Sprintf("service %s must declare a nonempty immutable service lineage", service.FullName()))
	}
	n.validateDocumentationURIs(location, fmt.Sprintf("service %s", service.FullName()), options.GetDocumentationUris())
	if options.GetInterfaceMajorVersion() > 0xff {
		n.diagnostics.error(ruleMember, location, fmt.Sprintf("service %s interface major version %d does not fit the SOME/IP 8-bit interface-version field", service.FullName(), options.GetInterfaceMajorVersion()))
	}
	options = canonicalServiceOptions(options)
	info := &serviceInfo{
		descriptor:          service,
		options:             options,
		methodMembers:       make(map[protoreflect.FullName]*manifestv1alpha1.ServiceMemberManifest),
		eventGroups:         make(map[string]uint32),
		payloads:            make(map[protoreflect.FullName]protoreflect.MessageDescriptor),
		memberPayloadBounds: make(map[uint32][]memberDirectionalBound),
		manifest: &manifestv1alpha1.ServiceManifest{
			Service:               cloneReference("." + string(service.FullName())),
			InterfaceMajorVersion: options.GetInterfaceMajorVersion(),
			InterfaceMinorVersion: options.GetInterfaceMinorVersion(),
			EventGroups:           canonicalEventGroups(options.GetEventGroups()),
			Location:              location.Manifest(),
			ServiceLineage:        options.GetServiceLineage(),
			DocumentationUris:     canonicalDocumentationURIs(options.GetDocumentationUris()),
		},
	}
	n.services[service.FullName()] = info

	memberIDs := make(map[uint32]string)
	methods := make(map[protoreflect.FullName]protoreflect.MethodDescriptor)
	for index := range service.Methods().Len() {
		method := service.Methods().Get(index)
		methods[method.FullName()] = method
		n.normalizeMethod(info, method, memberIDs)
	}
	n.normalizeEventGroups(info)
	n.normalizeEvents(info, memberIDs)
	n.normalizeFields(info, methods, memberIDs)
	n.validateEventGroupMembership(info)
	if len(info.manifest.Members) == 0 {
		n.diagnostics.error(rulePolicyService, location, fmt.Sprintf("annotated service %s must declare at least one dispatchable member", service.FullName()))
	}
	sort.Slice(info.manifest.Members, func(i, j int) bool {
		if info.manifest.Members[i].GetMemberId() != info.manifest.Members[j].GetMemberId() {
			return info.manifest.Members[i].GetMemberId() < info.manifest.Members[j].GetMemberId()
		}
		return info.manifest.Members[i].GetName() < info.manifest.Members[j].GetName()
	})
	sort.Slice(info.manifest.EventGroups, func(i, j int) bool {
		if info.manifest.EventGroups[i].GetId() != info.manifest.EventGroups[j].GetId() {
			return info.manifest.EventGroups[i].GetId() < info.manifest.EventGroups[j].GetId()
		}
		return info.manifest.EventGroups[i].GetName() < info.manifest.EventGroups[j].GetName()
	})
	sort.Slice(info.manifest.Fields, func(i, j int) bool {
		return info.manifest.Fields[i].GetField().GetName() < info.manifest.Fields[j].GetField().GetName()
	})
	return info
}

func (n *normalizer) normalizeMethod(
	service *serviceInfo,
	method protoreflect.MethodDescriptor,
	memberIDs map[uint32]string,
) {
	location := n.set.Locations.Descriptor(method)
	options := descriptor.MethodOptions(method)
	if method.IsStreamingClient() || method.IsStreamingServer() {
		n.diagnostics.error(rulePolicyRPC, location, fmt.Sprintf("RPC %s must be unary", method.FullName()))
	}
	if options == nil {
		n.diagnostics.error(rulePolicyRPC, location, fmt.Sprintf("RPC %s must declare explicit SOME/IP method metadata", method.FullName()))
		return
	}
	n.validateDocumentationURIs(location, fmt.Sprintf("RPC %s", method.FullName()), options.GetDocumentationUris())
	options = canonicalMethodOptions(options)
	validateMemberID(&n.diagnostics, location, options.GetMemberId(), false)
	if previous, found := memberIDs[options.GetMemberId()]; found {
		n.diagnostics.error(ruleID, location, fmt.Sprintf("member identifier %d collides with %s", options.GetMemberId(), previous))
	}
	memberIDs[options.GetMemberId()] = string(method.FullName())

	switch options.GetKind() {
	case optionsv1alpha1.MethodKind_METHOD_KIND_FIRE_AND_FORGET:
		if string(method.Output().FullName()) != "google.protobuf.Empty" {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("fire-and-forget RPC %s must return google.protobuf.Empty", method.FullName()))
		}
		if len(options.GetReturnCodes()) > 0 || options.GetExceptionPayloadType() != nil || options.GetResponsePolicy() != optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_UNSPECIFIED {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("fire-and-forget RPC %s cannot declare return-code, exception, or response-policy metadata", method.FullName()))
		}
	case optionsv1alpha1.MethodKind_METHOD_KIND_REQUEST_RESPONSE:
		n.validateMethodResults(method, options, location)
		if options.GetExceptionPayloadType() != nil {
			if exceptionPayload, resolved := n.resolveMessage(options.GetExceptionPayloadType(), location, ruleMember); resolved {
				if isEmpty(exceptionPayload) {
					n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s cannot use google.protobuf.Empty as an exception payload", method.FullName()))
				} else {
					n.recordPayload(service, exceptionPayload)
				}
			}
		}
	default:
		n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s must declare a supported method kind", method.FullName()))
	}
	n.validateMethodTiming(method, options.GetTiming(), location)

	member := &manifestv1alpha1.ServiceMemberManifest{
		Name:                string(method.Name()),
		MemberId:            options.GetMemberId(),
		Kind:                "method",
		RequestPayloadType:  cloneReference("." + string(method.Input().FullName())),
		ResponsePayloadType: cloneReference("." + string(method.Output().FullName())),
		Location:            location.Manifest(),
		MethodOptions:       options,
	}
	service.methodMembers[method.FullName()] = member
	service.manifest.Members = append(service.manifest.Members, member)
	n.recordPayload(service, method.Input())
	n.recordPayload(service, method.Output())
}

func (n *normalizer) validateMethodResults(
	method protoreflect.MethodDescriptor,
	options *optionsv1alpha1.MethodOptionsSpec,
	location descriptor.Location,
) {
	switch options.GetResponsePolicy() {
	case optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_RESPONSE_ONLY:
		if options.GetExceptionPayloadType() != nil {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s cannot declare an exception payload with RESPONSE_ONLY policy", method.FullName()))
		}
	case optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_EXCEPTION_ONLY,
		optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_RESPONSE_OR_EXCEPTION:
		if options.GetExceptionPayloadType() == nil {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s response policy requires an exception payload type", method.FullName()))
		}
	default:
		n.diagnostics.error(ruleMember, location, fmt.Sprintf("request/response RPC %s must declare a supported response policy", method.FullName()))
	}
	seen := make(map[uint32]struct{})
	names := make(map[string]struct{})
	for _, code := range options.GetReturnCodes() {
		if _, found := seen[code.GetCode()]; found {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s declares duplicate return code %d", method.FullName(), code.GetCode()))
		}
		seen[code.GetCode()] = struct{}{}
		if !validReturnCode(code.GetCode()) {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s return code %d is neither a generic code nor an application code in 0x20-0x3f", method.FullName(), code.GetCode()))
		}
		if code.GetName() == "" {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s return code %d must declare a stable symbolic name", method.FullName(), code.GetCode()))
		} else if _, found := names[code.GetName()]; found {
			n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s declares duplicate return-code name %q", method.FullName(), code.GetName()))
		}
		names[code.GetName()] = struct{}{}
	}
}

func (n *normalizer) validateMethodTiming(
	method protoreflect.MethodDescriptor,
	timing *optionsv1alpha1.MethodTiming,
	location descriptor.Location,
) {
	if timing == nil {
		return
	}
	if timing.GetResponseTimeout() != nil && !positiveDuration(timing.GetResponseTimeout()) {
		n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s response_timeout must be a positive valid duration", method.FullName()))
	}
	if timing.GetRetryDelay() != nil && !positiveDuration(timing.GetRetryDelay()) {
		n.diagnostics.error(ruleMember, location, fmt.Sprintf("RPC %s retry_delay must be a positive valid duration", method.FullName()))
	}
}

func validReturnCode(code uint32) bool {
	return code <= 0x0a || (code >= 0x20 && code <= 0x3f)
}

func (n *normalizer) normalizeEventGroups(service *serviceInfo) {
	seenNames := make(map[string]struct{})
	seenIDs := make(map[uint32]string)
	for _, group := range service.options.GetEventGroups() {
		location := n.set.Locations.Descriptor(service.descriptor)
		if group.GetName() == "" {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("service %s declares an event group with an empty name", service.descriptor.FullName()))
		}
		if _, found := seenNames[group.GetName()]; found {
			n.diagnostics.error(ruleID, location, fmt.Sprintf("event group name %q is duplicated", group.GetName()))
		}
		seenNames[group.GetName()] = struct{}{}
		validateEventGroupID(&n.diagnostics, location, group.GetId())
		if previous, found := seenIDs[group.GetId()]; found {
			n.diagnostics.error(ruleID, location, fmt.Sprintf("event group identifier %d collides with group %q", group.GetId(), previous))
		}
		seenIDs[group.GetId()] = group.GetName()
		service.eventGroups[group.GetName()] = 0
	}
}

func (n *normalizer) normalizeEvents(service *serviceInfo, memberIDs map[uint32]string) {
	seenNames := make(map[string]struct{})
	for _, event := range service.options.GetEvents() {
		location := n.set.Locations.Descriptor(service.descriptor)
		if event.GetName() == "" {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("service %s declares an event with an empty name", service.descriptor.FullName()))
		}
		if _, found := seenNames[event.GetName()]; found {
			n.diagnostics.error(ruleID, location, fmt.Sprintf("event name %q is duplicated", event.GetName()))
		}
		seenNames[event.GetName()] = struct{}{}
		validateMemberID(&n.diagnostics, location, event.GetId(), true)
		if previous, found := memberIDs[event.GetId()]; found {
			n.diagnostics.error(ruleID, location, fmt.Sprintf("event identifier %d collides with %s", event.GetId(), previous))
		}
		memberIDs[event.GetId()] = "event " + event.GetName()
		payload, resolved := n.resolveMessage(event.GetPayloadType(), location, ruleField)
		if resolved {
			if isEmpty(payload) {
				n.diagnostics.error(ruleField, location, fmt.Sprintf("event %s cannot use google.protobuf.Empty as its payload", event.GetName()))
			} else {
				n.recordPayload(service, payload)
			}
		}
		n.validateEventGroupReferences(service, event.GetEventGroupNames(), location, "event "+event.GetName())
		n.validatePublication(event.GetPublication(), payload, location, "event "+event.GetName())
		service.manifest.Members = append(service.manifest.Members, &manifestv1alpha1.ServiceMemberManifest{
			Name:                event.GetName(),
			MemberId:            event.GetId(),
			Kind:                "event",
			ResponsePayloadType: event.GetPayloadType(),
			EventGroupNames:     canonicalStrings(event.GetEventGroupNames()),
			Publication:         event.GetPublication(),
			Location:            location.Manifest(),
		})
	}
}

func (n *normalizer) normalizeFields(
	service *serviceInfo,
	methods map[protoreflect.FullName]protoreflect.MethodDescriptor,
	memberIDs map[uint32]string,
) {
	seenNames := make(map[string]struct{})
	for _, field := range service.options.GetFields() {
		field = canonicalServiceField(field)
		location := n.set.Locations.Descriptor(service.descriptor)
		service.manifest.Fields = append(service.manifest.Fields, &manifestv1alpha1.ServiceFieldManifest{
			Field:    field,
			Location: location.Manifest(),
		})
		if field.GetName() == "" {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("service %s declares a field with an empty name", service.descriptor.FullName()))
		}
		if _, found := seenNames[field.GetName()]; found {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("service field name %q is duplicated", field.GetName()))
		}
		seenNames[field.GetName()] = struct{}{}
		valueType, resolved := n.resolveMessage(field.GetValueType(), location, ruleField)
		if resolved {
			if isEmpty(valueType) {
				n.diagnostics.error(ruleField, location, fmt.Sprintf("field %q cannot use google.protobuf.Empty as its value type", field.GetName()))
				valueType = nil
			} else {
				n.recordPayload(service, valueType)
			}
		}
		if field.GetGetterMethod() == nil && field.GetSetterMethod() == nil && field.GetNotifierId() == 0 {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("field %q must declare at least one getter, setter, or notifier", field.GetName()))
		}
		n.validateFieldAccessor(service, field.GetName(), "getter", field.GetGetterMethod(), methods, valueType, true, location)
		n.validateFieldAccessor(service, field.GetName(), "setter", field.GetSetterMethod(), methods, valueType, false, location)
		if field.GetNotifierId() != 0 {
			validateMemberID(&n.diagnostics, location, field.GetNotifierId(), true)
			if previous, found := memberIDs[field.GetNotifierId()]; found {
				n.diagnostics.error(ruleID, location, fmt.Sprintf("field notifier identifier %d collides with %s", field.GetNotifierId(), previous))
			}
			memberIDs[field.GetNotifierId()] = "field notifier " + field.GetName()
			n.validateEventGroupReferences(service, field.GetEventGroupNames(), location, "field notifier "+field.GetName())
			n.validatePublication(field.GetPublication(), valueType, location, "field notifier "+field.GetName())
			service.manifest.Members = append(service.manifest.Members, &manifestv1alpha1.ServiceMemberManifest{
				Name:                field.GetName() + "_notifier",
				MemberId:            field.GetNotifierId(),
				Kind:                "field_notifier",
				ResponsePayloadType: field.GetValueType(),
				EventGroupNames:     canonicalStrings(field.GetEventGroupNames()),
				Publication:         field.GetPublication(),
				Location:            location.Manifest(),
			})
		} else if field.GetPublication() != nil || len(field.GetEventGroupNames()) > 0 {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("field %q declares notifier metadata without a notifier identifier", field.GetName()))
		}
	}
}

func (n *normalizer) validateFieldAccessor(
	service *serviceInfo,
	fieldName string,
	accessorKind string,
	reference *optionsv1alpha1.ProtobufReference,
	methods map[protoreflect.FullName]protoreflect.MethodDescriptor,
	valueType protoreflect.MessageDescriptor,
	isGetter bool,
	location descriptor.Location,
) {
	if reference == nil {
		return
	}
	name, ok := normalizedFQN(reference)
	if !ok {
		n.diagnostics.error(ruleField, location, fmt.Sprintf("field %q %s reference must use a leading-dot fully qualified method name", fieldName, accessorKind))
		return
	}
	method, found := methods[name]
	if !found {
		n.diagnostics.error(ruleField, location, fmt.Sprintf("field %q %s %q is not declared by service %s", fieldName, accessorKind, reference.GetFqn(), service.descriptor.FullName()))
		return
	}
	if valueType != nil {
		if isGetter {
			if !isEmpty(method.Input()) || method.Output().FullName() != valueType.FullName() {
				n.diagnostics.error(ruleField, location, fmt.Sprintf("getter %s must have signature google.protobuf.Empty -> %s", method.FullName(), valueType.FullName()))
			}
		} else if method.Input().FullName() != valueType.FullName() || method.Output().FullName() != valueType.FullName() {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("setter %s must have signature %s -> %s", method.FullName(), valueType.FullName(), valueType.FullName()))
		}
	}
	if options := descriptor.MethodOptions(method); options == nil || options.GetKind() != optionsv1alpha1.MethodKind_METHOD_KIND_REQUEST_RESPONSE {
		n.diagnostics.error(ruleField, location, fmt.Sprintf("field %q %s %s must be a request/response RPC", fieldName, accessorKind, method.FullName()))
	}
	if member, found := service.methodMembers[name]; found {
		if isGetter {
			member.Kind = "field_getter"
		} else {
			member.Kind = "field_setter"
		}
	}
}

func (n *normalizer) validateEventGroupReferences(
	service *serviceInfo,
	names []string,
	location descriptor.Location,
	member string,
) {
	seen := make(map[string]struct{})
	for _, name := range names {
		if _, found := seen[name]; found {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("%s repeats event group %q", member, name))
		}
		seen[name] = struct{}{}
		if _, found := service.eventGroups[name]; !found {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("%s references undeclared event group %q", member, name))
			continue
		}
		service.eventGroups[name]++
	}
}

func (n *normalizer) validateEventGroupMembership(service *serviceInfo) {
	location := n.set.Locations.Descriptor(service.descriptor)
	for name, members := range service.eventGroups {
		if members == 0 {
			n.diagnostics.error(rulePolicyService, location, fmt.Sprintf("event group %q in service %s has no member", name, service.descriptor.FullName()))
		}
	}
}

func (n *normalizer) validatePublication(
	publication *optionsv1alpha1.PublicationPolicy,
	payload protoreflect.MessageDescriptor,
	location descriptor.Location,
	member string,
) {
	if publication == nil {
		n.diagnostics.error(ruleField, location, fmt.Sprintf("%s must declare an explicit publication policy", member))
		return
	}
	switch publication.GetKind() {
	case optionsv1alpha1.PublicationPolicyKind_PUBLICATION_POLICY_KIND_EVENT_DRIVEN,
		optionsv1alpha1.PublicationPolicyKind_PUBLICATION_POLICY_KIND_ON_CHANGE:
		if publication.GetCycle() != nil {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("%s cannot declare a cycle for its publication policy", member))
		}
		if publication.GetEpsilon() != nil {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("%s cannot declare epsilon outside EPSILON_CHANGE publication", member))
		}
	case optionsv1alpha1.PublicationPolicyKind_PUBLICATION_POLICY_KIND_CYCLIC:
		if !positiveDuration(publication.GetCycle()) {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("cyclic %s must declare a positive cycle duration", member))
		}
		if publication.GetEpsilon() != nil {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("cyclic %s cannot declare epsilon", member))
		}
	case optionsv1alpha1.PublicationPolicyKind_PUBLICATION_POLICY_KIND_EPSILON_CHANGE:
		if publication.GetCycle() != nil {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("epsilon-change %s cannot declare a cycle", member))
		}
		if publication.GetEpsilon() == nil || publication.GetEpsilon().GetDenominator() == 0 || publication.GetEpsilon().GetNumerator() <= 0 {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("epsilon-change %s must declare a positive exact rational epsilon", member))
		}
		if !isSingleNumericPayload(payload) {
			n.diagnostics.error(ruleField, location, fmt.Sprintf("epsilon-change %s requires a payload with exactly one numeric scalar member", member))
		}
	default:
		n.diagnostics.error(ruleField, location, fmt.Sprintf("%s must declare a supported publication policy", member))
	}
	if publication.GetDebounce() != nil && !positiveDuration(publication.GetDebounce()) {
		n.diagnostics.error(ruleField, location, fmt.Sprintf("%s debounce duration must be positive when declared", member))
	}
	if publication.GetRetention() != nil && !positiveDuration(publication.GetRetention()) {
		n.diagnostics.error(ruleField, location, fmt.Sprintf("%s retention duration must be positive when declared", member))
	}
}

func (n *normalizer) resolveMessage(
	reference *optionsv1alpha1.ProtobufReference,
	location descriptor.Location,
	rule rule,
) (protoreflect.MessageDescriptor, bool) {
	name, ok := normalizedFQN(reference)
	if !ok {
		n.diagnostics.error(rule, location, "payload reference must use a leading-dot fully qualified protobuf message name")
		return nil, false
	}
	descriptor, err := n.set.Files.FindDescriptorByName(name)
	if err != nil {
		n.diagnostics.error(rule, location, fmt.Sprintf("payload reference %q cannot be resolved", reference.GetFqn()))
		return nil, false
	}
	message, ok := descriptor.(protoreflect.MessageDescriptor)
	if !ok {
		n.diagnostics.error(rule, location, fmt.Sprintf("reference %q does not name a protobuf message", reference.GetFqn()))
		return nil, false
	}
	return message, true
}

func (n *normalizer) recordPayload(service *serviceInfo, message protoreflect.MessageDescriptor) {
	if message == nil || isEmpty(message) {
		return
	}
	service.payloads[message.FullName()] = message
}

func isEmpty(message protoreflect.MessageDescriptor) bool {
	return message != nil && message.FullName() == "google.protobuf.Empty"
}

func positiveDuration(duration interface {
	GetSeconds() int64
	GetNanos() int32
}) bool {
	if duration == nil {
		return false
	}
	seconds, nanos := duration.GetSeconds(), duration.GetNanos()
	if seconds < -315576000000 || seconds > 315576000000 || nanos < -999999999 || nanos > 999999999 {
		return false
	}
	if (seconds > 0 && nanos < 0) || (seconds < 0 && nanos > 0) {
		return false
	}
	return seconds > 0 || (seconds == 0 && nanos > 0)
}

func isSingleNumericPayload(message protoreflect.MessageDescriptor) bool {
	if message == nil || descriptor.MessageOptions(message) == nil || message.Fields().Len() != 1 || message.Oneofs().Len() != 0 {
		return false
	}
	field := message.Fields().Get(0)
	if field.Cardinality() != protoreflect.Optional || field.HasOptionalKeyword() {
		return false
	}
	switch field.Kind() {
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.FloatKind, protoreflect.DoubleKind:
		return true
	default:
		return false
	}
}
