package descriptor

import (
	"strconv"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Location identifies a descriptor without storing an absolute source path.
type Location struct {
	ImportPath string
	Line       uint32
	Column     uint32
}

// Manifest converts Location to the public manifest representation.
func (l Location) Manifest() *manifestv1alpha1.SourceLocation {
	return &manifestv1alpha1.SourceLocation{
		ImportPath: l.ImportPath,
		Line:       l.Line,
		Column:     l.Column,
	}
}

// Locations indexes SourceCodeInfo embedded in a FileDescriptorSet.
type Locations struct {
	byPath       map[string]map[string]Location
	byDescriptor map[string]Location
}

func newLocations(set *descriptorpb.FileDescriptorSet) *Locations {
	locations := &Locations{
		byPath:       make(map[string]map[string]Location, len(set.File)),
		byDescriptor: make(map[string]Location),
	}
	for _, file := range set.File {
		byPath := make(map[string]Location)
		for _, location := range file.GetSourceCodeInfo().GetLocation() {
			line, column := uint32(1), uint32(1)
			if span := location.GetSpan(); len(span) >= 2 {
				line = uint32(span[0]) + 1
				column = uint32(span[1]) + 1
			}
			key := pathKey(location.GetPath())
			candidate := Location{
				ImportPath: file.GetName(),
				Line:       line,
				Column:     column,
			}
			if current, found := byPath[key]; !found || locationBefore(candidate, current) {
				byPath[key] = candidate
			}
		}
		locations.byPath[file.GetName()] = byPath
		locations.indexFile(file)
	}
	return locations
}

func locationBefore(left, right Location) bool {
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.Column < right.Column
}

// Path resolves a logical source location for a descriptor-proto path. It
// falls back through ancestors to retain useful locations for generated FDSs
// that omit a fine-grained span.
func (l *Locations) Path(importPath string, path []int32) Location {
	locations := l.byPath[importPath]
	for len(path) >= 0 {
		if location, found := locations[pathKey(path)]; found {
			return location
		}
		if len(path) == 0 {
			break
		}
		path = path[:len(path)-1]
	}
	return Location{ImportPath: importPath, Line: 1, Column: 1}
}

// FileOptions returns the source location of a file's option block.
func (l *Locations) FileOptions(importPath string) Location {
	return l.Path(importPath, []int32{8})
}

// Descriptor returns the descriptor declaration location.
func (l *Locations) Descriptor(descriptor protoreflect.Descriptor) Location {
	if descriptor == nil {
		return Location{}
	}
	importPath := string(descriptor.ParentFile().Path())
	if path, found := descriptorSourcePath(descriptor); found {
		return l.Path(importPath, path)
	}
	kind, name := descriptorIdentity(descriptor)
	if location, found := l.byDescriptor[descriptorKey(kind, name)]; found {
		return location
	}
	return l.Path(importPath, nil)
}

func descriptorSourcePath(descriptor protoreflect.Descriptor) ([]int32, bool) {
	switch descriptor := descriptor.(type) {
	case protoreflect.FileDescriptor:
		return nil, true
	case protoreflect.FieldDescriptor:
		parentPath, found := descriptorSourcePath(descriptor.Parent())
		if !found {
			return nil, false
		}
		fieldNumber := int32(2)
		if descriptor.IsExtension() {
			switch descriptor.Parent().(type) {
			case protoreflect.FileDescriptor:
				fieldNumber = 7
			case protoreflect.MessageDescriptor:
				fieldNumber = 6
			default:
				return nil, false
			}
		}
		return appendPath(parentPath, fieldNumber, int32(descriptor.Index())), true
	case protoreflect.OneofDescriptor:
		parentPath, found := descriptorSourcePath(descriptor.Parent())
		return appendPath(parentPath, 8, int32(descriptor.Index())), found
	case protoreflect.MessageDescriptor:
		parentPath, found := descriptorSourcePath(descriptor.Parent())
		if !found {
			return nil, false
		}
		switch descriptor.Parent().(type) {
		case protoreflect.FileDescriptor:
			return appendPath(parentPath, 4, int32(descriptor.Index())), true
		case protoreflect.MessageDescriptor:
			return appendPath(parentPath, 3, int32(descriptor.Index())), true
		default:
			return nil, false
		}
	case protoreflect.EnumDescriptor:
		parentPath, found := descriptorSourcePath(descriptor.Parent())
		if !found {
			return nil, false
		}
		switch descriptor.Parent().(type) {
		case protoreflect.FileDescriptor:
			return appendPath(parentPath, 5, int32(descriptor.Index())), true
		case protoreflect.MessageDescriptor:
			return appendPath(parentPath, 4, int32(descriptor.Index())), true
		default:
			return nil, false
		}
	case protoreflect.EnumValueDescriptor:
		parentPath, found := descriptorSourcePath(descriptor.Parent())
		return appendPath(parentPath, 2, int32(descriptor.Index())), found
	case protoreflect.ServiceDescriptor:
		parentPath, found := descriptorSourcePath(descriptor.Parent())
		return appendPath(parentPath, 6, int32(descriptor.Index())), found
	case protoreflect.MethodDescriptor:
		parentPath, found := descriptorSourcePath(descriptor.Parent())
		return appendPath(parentPath, 2, int32(descriptor.Index())), found
	default:
		return nil, false
	}
}

func (l *Locations) indexFile(file *descriptorpb.FileDescriptorProto) {
	packageName := file.GetPackage()
	l.index("file", file.GetName(), file.GetName(), nil)
	type messageFrame struct {
		message  *descriptorpb.DescriptorProto
		path     []int32
		fullName string
	}
	frames := make([]messageFrame, 0, len(file.GetMessageType()))
	for index, message := range file.MessageType {
		frames = append(frames, messageFrame{
			message:  message,
			path:     []int32{4, int32(index)},
			fullName: qualifiedName(packageName, message.GetName()),
		})
	}
	for index, enum := range file.EnumType {
		l.indexEnum(file.GetName(), enum, []int32{5, int32(index)}, qualifiedName(packageName, enum.GetName()))
	}
	for index, service := range file.Service {
		serviceName := qualifiedName(packageName, service.GetName())
		servicePath := []int32{6, int32(index)}
		l.index("service", serviceName, file.GetName(), servicePath)
		for methodIndex, method := range service.Method {
			l.index("method", qualifiedName(serviceName, method.GetName()), file.GetName(), appendPath(servicePath, 2, int32(methodIndex)))
		}
	}
	for index, extension := range file.Extension {
		l.index("extension", qualifiedName(packageName, extension.GetName()), file.GetName(), []int32{7, int32(index)})
	}
	for len(frames) > 0 {
		last := len(frames) - 1
		frame := frames[last]
		frames = frames[:last]
		message := frame.message
		l.index("message", frame.fullName, file.GetName(), frame.path)
		for index, field := range message.GetField() {
			l.index("field", qualifiedName(frame.fullName, field.GetName()), file.GetName(), appendPath(frame.path, 2, int32(index)))
		}
		for index, extension := range message.GetExtension() {
			l.index("extension", qualifiedName(frame.fullName, extension.GetName()), file.GetName(), appendPath(frame.path, 6, int32(index)))
		}
		for index, oneof := range message.GetOneofDecl() {
			l.index("oneof", qualifiedName(frame.fullName, oneof.GetName()), file.GetName(), appendPath(frame.path, 8, int32(index)))
		}
		for index, nested := range message.GetNestedType() {
			frames = append(frames, messageFrame{
				message:  nested,
				path:     appendPath(frame.path, 3, int32(index)),
				fullName: qualifiedName(frame.fullName, nested.GetName()),
			})
		}
		for index, enum := range message.GetEnumType() {
			l.indexEnum(file.GetName(), enum, appendPath(frame.path, 4, int32(index)), qualifiedName(frame.fullName, enum.GetName()))
		}
	}
}

func (l *Locations) indexEnum(
	importPath string,
	enum *descriptorpb.EnumDescriptorProto,
	path []int32,
	fullName string,
) {
	l.index("enum", fullName, importPath, path)
	for index, value := range enum.Value {
		l.index("enum_value", qualifiedName(fullName, value.GetName()), importPath, appendPath(path, 2, int32(index)))
	}
}

func (l *Locations) index(kind, name, importPath string, path []int32) {
	l.byDescriptor[descriptorKey(kind, name)] = l.Path(importPath, path)
}

func descriptorIdentity(descriptor protoreflect.Descriptor) (string, string) {
	switch descriptor := descriptor.(type) {
	case protoreflect.FileDescriptor:
		return "file", string(descriptor.Path())
	case protoreflect.FieldDescriptor:
		if descriptor.IsExtension() {
			return "extension", string(descriptor.FullName())
		}
		return "field", string(descriptor.FullName())
	case protoreflect.OneofDescriptor:
		return "oneof", string(descriptor.FullName())
	case protoreflect.MessageDescriptor:
		return "message", string(descriptor.FullName())
	case protoreflect.EnumDescriptor:
		return "enum", string(descriptor.FullName())
	case protoreflect.EnumValueDescriptor:
		return "enum_value", string(descriptor.FullName())
	case protoreflect.ServiceDescriptor:
		return "service", string(descriptor.FullName())
	case protoreflect.MethodDescriptor:
		return "method", string(descriptor.FullName())
	default:
		return "", string(descriptor.FullName())
	}
}

func descriptorKey(kind, name string) string {
	return kind + ":" + name
}

func qualifiedName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func pathKey(path []int32) string {
	if len(path) == 0 {
		return ""
	}
	parts := make([]string, len(path))
	for index, value := range path {
		parts[index] = strconv.FormatInt(int64(value), 10)
	}
	return strings.Join(parts, "/")
}
