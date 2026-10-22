package descriptor

import (
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FileOptions returns the project file option when present.
func FileOptions(file protoreflect.FileDescriptor) *optionsv1alpha1.FileOptionsSpec {
	options, _ := file.Options().(*descriptorpb.FileOptions)
	return getExtension[*optionsv1alpha1.FileOptionsSpec](options, optionsv1alpha1.E_File)
}

// ServiceOptions returns the project service option when present.
func ServiceOptions(service protoreflect.ServiceDescriptor) *optionsv1alpha1.ServiceOptionsSpec {
	options, _ := service.Options().(*descriptorpb.ServiceOptions)
	return getExtension[*optionsv1alpha1.ServiceOptionsSpec](options, optionsv1alpha1.E_Service)
}

// MethodOptions returns the project method option when present.
func MethodOptions(method protoreflect.MethodDescriptor) *optionsv1alpha1.MethodOptionsSpec {
	options, _ := method.Options().(*descriptorpb.MethodOptions)
	return getExtension[*optionsv1alpha1.MethodOptionsSpec](options, optionsv1alpha1.E_Method)
}

// MessageOptions returns the project message option when present.
func MessageOptions(message protoreflect.MessageDescriptor) *optionsv1alpha1.MessageOptionsSpec {
	options, _ := message.Options().(*descriptorpb.MessageOptions)
	return getExtension[*optionsv1alpha1.MessageOptionsSpec](options, optionsv1alpha1.E_Message)
}

// FieldOptions returns the project field option when present.
func FieldOptions(field protoreflect.FieldDescriptor) *optionsv1alpha1.FieldOptionsSpec {
	options, _ := field.Options().(*descriptorpb.FieldOptions)
	return getExtension[*optionsv1alpha1.FieldOptionsSpec](options, optionsv1alpha1.E_Field)
}

// OneofOptions returns the project oneof option when present.
func OneofOptions(oneof protoreflect.OneofDescriptor) *optionsv1alpha1.OneofOptionsSpec {
	options, _ := oneof.Options().(*descriptorpb.OneofOptions)
	return getExtension[*optionsv1alpha1.OneofOptionsSpec](options, optionsv1alpha1.E_Oneof)
}

// EnumOptions returns the project enum option when present.
func EnumOptions(enum protoreflect.EnumDescriptor) *optionsv1alpha1.EnumOptionsSpec {
	options, _ := enum.Options().(*descriptorpb.EnumOptions)
	return getExtension[*optionsv1alpha1.EnumOptionsSpec](options, optionsv1alpha1.E_Enum)
}

// EnumValueOptions returns the project enum-value option when present.
func EnumValueOptions(value protoreflect.EnumValueDescriptor) *optionsv1alpha1.EnumValueOptionsSpec {
	options, _ := value.Options().(*descriptorpb.EnumValueOptions)
	return getExtension[*optionsv1alpha1.EnumValueOptionsSpec](options, optionsv1alpha1.E_EnumValue)
}

func getExtension[T any](options proto.Message, extension protoreflect.ExtensionType) T {
	var zero T
	if options == nil || !proto.HasExtension(options, extension) {
		return zero
	}
	value := proto.GetExtension(options, extension)
	result, ok := value.(T)
	if !ok {
		return zero
	}
	return result
}
