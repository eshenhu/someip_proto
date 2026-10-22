package descriptor

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ErrorKind classifies failures at the untrusted FDS boundary.
type ErrorKind string

const (
	// ErrorMalformed identifies an incomplete, conflicting, or invalid FDS.
	ErrorMalformed ErrorKind = "malformed"
	// ErrorLimit identifies a resource-limit violation.
	ErrorLimit ErrorKind = "limit"
)

// Error is returned for malformed or over-limit FileDescriptorSet input.
type Error struct {
	Kind ErrorKind
	Err  error
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func malformed(format string, args ...any) error {
	return &Error{Kind: ErrorMalformed, Err: fmt.Errorf(format, args...)}
}

func overLimit(format string, args ...any) error {
	return &Error{Kind: ErrorLimit, Err: fmt.Errorf(format, args...)}
}

// IsInputError reports whether err was caused by malformed or over-limit FDS
// input rather than a semantic validation failure.
func IsInputError(err error) bool {
	var inputErr *Error
	return errors.As(err, &inputErr)
}

// Set is a reconstructed, source-aware descriptor registry. It never refers to
// the original source files.
type Set struct {
	Files          *protoregistry.Files
	Raw            *descriptorpb.FileDescriptorSet
	InputSHA256    string
	Locations      *Locations
	UnknownOptions []*manifestv1alpha1.UnknownOption

	filesByPath map[string]protoreflect.FileDescriptor
	origins     map[string]FileOrigin
	limits      Limits
}

// FileByPath returns a reconstructed file descriptor by its logical import
// path.
func (s *Set) FileByPath(path string) (protoreflect.FileDescriptor, bool) {
	file, ok := s.filesByPath[path]
	return file, ok
}

// Origin reports the trusted provenance classification for file.
func (s *Set) Origin(file protoreflect.FileDescriptor) FileOrigin {
	if file == nil {
		return FileOriginUnknown
	}
	return s.OriginByPath(string(file.Path()))
}

// OriginByPath reports the trusted provenance classification for a logical
// import path.
func (s *Set) OriginByPath(importPath string) FileOrigin {
	if s == nil {
		return FileOriginUnknown
	}
	if origin, found := s.origins[importPath]; found {
		return origin
	}
	return FileOriginUnknown
}

// AllFiles returns reconstructed files in deterministic import-path order.
func (s *Set) AllFiles() []protoreflect.FileDescriptor {
	paths := make([]string, 0, len(s.filesByPath))
	for path := range s.filesByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	files := make([]protoreflect.FileDescriptor, 0, len(paths))
	for _, path := range paths {
		files = append(files, s.filesByPath[path])
	}
	return files
}

// Limits returns the normalized resource limits applied when loading the set.
func (s *Set) Limits() Limits {
	if s == nil {
		return DefaultLimits()
	}
	return s.limits.normalized()
}

// Load parses and strictly reconstructs a complete, source-aware binary
// FileDescriptorSet. It performs no filesystem or network access.
func Load(input []byte, limits Limits) (*Set, error) {
	limits = limits.normalized()
	if len(input) == 0 {
		return nil, malformed("FileDescriptorSet input is empty")
	}
	if len(input) > limits.MaxInputBytes {
		return nil, overLimit("FileDescriptorSet is %d bytes, exceeding the %d-byte limit", len(input), limits.MaxInputBytes)
	}

	set := &descriptorpb.FileDescriptorSet{}
	if err := (proto.UnmarshalOptions{
		AllowPartial:   false,
		RecursionLimit: limits.protobufRecursionLimit(),
	}).Unmarshal(input, set); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "recursion") {
			return nil, overLimit("decode FileDescriptorSet: nesting exceeds the %d-level limit", limits.MaxDescriptorDepth)
		}
		return nil, malformed("decode FileDescriptorSet: %w", err)
	}
	if len(set.File) == 0 {
		return nil, malformed("FileDescriptorSet contains no files")
	}
	if len(set.File) > limits.MaxFiles {
		return nil, overLimit("FileDescriptorSet contains %d files, exceeding the %d-file limit", len(set.File), limits.MaxFiles)
	}

	byPath := make(map[string]*descriptorpb.FileDescriptorProto, len(set.File))
	sourceLocations := 0
	for _, file := range set.File {
		if file == nil {
			return nil, malformed("FileDescriptorSet contains a nil file descriptor")
		}
		if !validImportPath(file.GetName()) {
			return nil, malformed("FileDescriptorSet contains non-logical import path %q", file.GetName())
		}
		if _, exists := byPath[file.GetName()]; exists {
			return nil, malformed("FileDescriptorSet contains duplicate file %q", file.GetName())
		}
		if err := validateSourceCodeInfo(file, limits, &sourceLocations); err != nil {
			return nil, err
		}
		byPath[file.GetName()] = file
	}
	if err := validateActiveOptionsAPI(set); err != nil {
		return nil, err
	}

	descriptorCount := 0
	for _, file := range set.File {
		count, err := countFileDescriptors(file, limits.MaxDescriptorDepth)
		if err != nil {
			return nil, err
		}
		descriptorCount += count
		if descriptorCount > limits.MaxDescriptors {
			return nil, overLimit("FileDescriptorSet contains more than %d descriptors", limits.MaxDescriptors)
		}
		for _, dependency := range file.Dependency {
			if _, found := byPath[dependency]; !found {
				return nil, malformed("file %q imports missing dependency %q", file.GetName(), dependency)
			}
		}
	}

	files, filesByPath, err := reconstruct(byPath)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(input)
	loaded := &Set{
		Files:       files,
		Raw:         set,
		InputSHA256: fmt.Sprintf("%x", sum),
		Locations:   newLocations(set),
		filesByPath: filesByPath,
		origins:     classifyFileOrigins(set),
		limits:      limits,
	}
	loaded.UnknownOptions = collectUnknownOptions(set, loaded.Locations, extensionNames(files))
	return loaded, nil
}

func validateActiveOptionsAPI(set *descriptorpb.FileDescriptorSet) error {
	if !hasProjectOptions(set) {
		return nil
	}
	expected := activeOptionsAPIDescriptors()
	actual := make(map[string]*descriptorpb.FileDescriptorProto, len(expected))
	for _, file := range set.GetFile() {
		if _, required := expected[file.GetName()]; required {
			actual[file.GetName()] = file
		}
	}
	paths := make([]string, 0, len(expected))
	for importPath := range expected {
		paths = append(paths, importPath)
	}
	sort.Strings(paths)
	for _, importPath := range paths {
		file := actual[importPath]
		if file == nil {
			return malformed("annotated FileDescriptorSet does not include the active options API descriptor %q", importPath)
		}
		file = proto.Clone(file).(*descriptorpb.FileDescriptorProto)
		file.SourceCodeInfo = nil
		if !proto.Equal(file, expected[importPath]) {
			return malformed("FileDescriptorSet options API descriptor %q does not match the active v1alpha1 API", importPath)
		}
	}
	return nil
}

func activeOptionsAPIDescriptors() map[string]*descriptorpb.FileDescriptorProto {
	files := []protoreflect.FileDescriptor{
		optionsv1alpha1.File_io_github_someip_proto_options_v1alpha1_someip_options_proto,
		optionsv1alpha1.File_io_github_someip_proto_options_v1alpha1_someip_common_options_proto,
		optionsv1alpha1.File_io_github_someip_proto_options_v1alpha1_someip_deployment_options_proto,
		optionsv1alpha1.File_io_github_someip_proto_options_v1alpha1_someip_sd_options_proto,
		optionsv1alpha1.File_io_github_someip_proto_options_v1alpha1_someip_tp_options_proto,
	}
	descriptors := make(map[string]*descriptorpb.FileDescriptorProto, len(files))
	for _, file := range files {
		descriptor := protodesc.ToFileDescriptorProto(file)
		descriptors[descriptor.GetName()] = descriptor
	}
	return descriptors
}

func hasProjectOptions(set *descriptorpb.FileDescriptorSet) bool {
	for _, file := range set.GetFile() {
		if hasOptionExtension(file.GetOptions(), optionsv1alpha1.E_File) {
			return true
		}
		for _, service := range file.GetService() {
			if hasOptionExtension(service.GetOptions(), optionsv1alpha1.E_Service) {
				return true
			}
			for _, method := range service.GetMethod() {
				if hasOptionExtension(method.GetOptions(), optionsv1alpha1.E_Method) {
					return true
				}
			}
		}
		for _, enum := range file.GetEnumType() {
			if hasOptionExtension(enum.GetOptions(), optionsv1alpha1.E_Enum) {
				return true
			}
			for _, value := range enum.GetValue() {
				if hasOptionExtension(value.GetOptions(), optionsv1alpha1.E_EnumValue) {
					return true
				}
			}
		}
		messages := append([]*descriptorpb.DescriptorProto(nil), file.GetMessageType()...)
		for len(messages) > 0 {
			last := len(messages) - 1
			message := messages[last]
			messages = messages[:last]
			if message == nil {
				continue
			}
			if hasOptionExtension(message.GetOptions(), optionsv1alpha1.E_Message) {
				return true
			}
			for _, field := range message.GetField() {
				if hasOptionExtension(field.GetOptions(), optionsv1alpha1.E_Field) {
					return true
				}
			}
			for _, oneof := range message.GetOneofDecl() {
				if hasOptionExtension(oneof.GetOptions(), optionsv1alpha1.E_Oneof) {
					return true
				}
			}
			for _, enum := range message.GetEnumType() {
				if hasOptionExtension(enum.GetOptions(), optionsv1alpha1.E_Enum) {
					return true
				}
				for _, value := range enum.GetValue() {
					if hasOptionExtension(value.GetOptions(), optionsv1alpha1.E_EnumValue) {
						return true
					}
				}
			}
			messages = append(messages, message.GetNestedType()...)
		}
	}
	return false
}

func hasOptionExtension(options proto.Message, extension protoreflect.ExtensionType) bool {
	return options != nil && proto.HasExtension(options, extension)
}

func validImportPath(importPath string) bool {
	if importPath == "" || path.IsAbs(importPath) || strings.Contains(importPath, "\\") || strings.Contains(importPath, ":") {
		return false
	}
	for _, character := range importPath {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	if strings.Contains(importPath, "://") {
		return false
	}
	return path.Clean(importPath) == importPath && importPath != "." && !strings.HasPrefix(importPath, "../")
}

func validateSourceCodeInfo(file *descriptorpb.FileDescriptorProto, limits Limits, total *int) error {
	locations := file.GetSourceCodeInfo().GetLocation()
	if len(locations) == 0 {
		return malformed("file %q has no SourceCodeInfo", file.GetName())
	}
	*total += len(locations)
	if *total > limits.MaxSourceLocations {
		return overLimit("FileDescriptorSet contains more than %d SourceCodeInfo locations", limits.MaxSourceLocations)
	}

	hasRoot := false
	for index, location := range locations {
		if location == nil {
			return malformed("file %q contains a nil SourceCodeInfo location", file.GetName())
		}
		locationPath := location.GetPath()
		if len(locationPath) > limits.MaxSourcePathElements {
			return overLimit("file %q contains a SourceCodeInfo path longer than the %d-element limit", file.GetName(), limits.MaxSourcePathElements)
		}
		for _, element := range locationPath {
			if element < 0 {
				return malformed("file %q contains a negative SourceCodeInfo path element", file.GetName())
			}
		}
		if len(locationPath) == 0 {
			hasRoot = true
		}
		if err := validateSourceSpan(file.GetName(), index, location.GetSpan()); err != nil {
			return err
		}
	}
	if !hasRoot {
		return malformed("file %q has no root SourceCodeInfo location", file.GetName())
	}
	return validateDeclarationSourceLocations(file, locations)
}

func validateSourceSpan(fileName string, index int, span []int32) error {
	if len(span) != 3 && len(span) != 4 {
		return malformed("file %q SourceCodeInfo location %d has an invalid %d-element span", fileName, index, len(span))
	}
	for _, coordinate := range span {
		if coordinate < 0 {
			return malformed("file %q SourceCodeInfo location %d has a negative span coordinate", fileName, index)
		}
	}
	startLine, startColumn := span[0], span[1]
	endLine, endColumn := startLine, span[2]
	if len(span) == 4 {
		endLine, endColumn = span[2], span[3]
	}
	if endLine < startLine || (endLine == startLine && endColumn < startColumn) {
		return malformed("file %q SourceCodeInfo location %d has a reversed span", fileName, index)
	}
	return nil
}

func validateDeclarationSourceLocations(
	file *descriptorpb.FileDescriptorProto,
	locations []*descriptorpb.SourceCodeInfo_Location,
) error {
	byPath := make(map[string][]*descriptorpb.SourceCodeInfo_Location, len(locations))
	for _, location := range locations {
		byPath[pathKey(location.GetPath())] = append(byPath[pathKey(location.GetPath())], location)
	}
	expected := declarationSourcePaths(file)
	for _, declaration := range expected {
		matches := byPath[pathKey(declaration.path)]
		if len(matches) == 0 {
			return malformed("file %q has no exact SourceCodeInfo location for %s", file.GetName(), declaration.name)
		}
		for _, duplicate := range matches[1:] {
			if !proto.Equal(matches[0], duplicate) {
				return malformed("file %q has ambiguous SourceCodeInfo locations for %s", file.GetName(), declaration.name)
			}
		}
	}
	return nil
}

type declarationSourcePath struct {
	path []int32
	name string
}

func declarationSourcePaths(file *descriptorpb.FileDescriptorProto) []declarationSourcePath {
	paths := make([]declarationSourcePath, 0)
	add := func(path []int32, name string) {
		paths = append(paths, declarationSourcePath{path: append([]int32(nil), path...), name: name})
	}
	add(nil, "file declaration")
	for index, enum := range file.GetEnumType() {
		addEnumDeclarationSourcePaths(&paths, enum, []int32{5, int32(index)})
	}
	for index, service := range file.GetService() {
		servicePath := []int32{6, int32(index)}
		add(servicePath, "service declaration")
		if service == nil {
			continue
		}
		for methodIndex := range service.GetMethod() {
			add(appendPath(servicePath, 2, int32(methodIndex)), "method declaration")
		}
	}
	for index := range file.GetExtension() {
		add([]int32{7, int32(index)}, "extension declaration")
	}
	type messageFrame struct {
		message *descriptorpb.DescriptorProto
		path    []int32
	}
	frames := make([]messageFrame, 0, len(file.GetMessageType()))
	for index, message := range file.GetMessageType() {
		frames = append(frames, messageFrame{message: message, path: []int32{4, int32(index)}})
	}
	for len(frames) > 0 {
		last := len(frames) - 1
		frame := frames[last]
		frames = frames[:last]
		add(frame.path, "message declaration")
		if frame.message == nil {
			continue
		}
		for index := range frame.message.GetField() {
			add(appendPath(frame.path, 2, int32(index)), "field declaration")
		}
		for index, enum := range frame.message.GetEnumType() {
			addEnumDeclarationSourcePaths(&paths, enum, appendPath(frame.path, 4, int32(index)))
		}
		for index := range frame.message.GetExtension() {
			add(appendPath(frame.path, 6, int32(index)), "extension declaration")
		}
		for index := range frame.message.GetOneofDecl() {
			add(appendPath(frame.path, 8, int32(index)), "oneof declaration")
		}
		for index, nested := range frame.message.GetNestedType() {
			frames = append(frames, messageFrame{message: nested, path: appendPath(frame.path, 3, int32(index))})
		}
	}
	return paths
}

func addEnumDeclarationSourcePaths(paths *[]declarationSourcePath, enum *descriptorpb.EnumDescriptorProto, path []int32) {
	*paths = append(*paths, declarationSourcePath{path: append([]int32(nil), path...), name: "enum declaration"})
	if enum == nil {
		return
	}
	for index := range enum.GetValue() {
		*paths = append(*paths, declarationSourcePath{
			path: appendPath(path, 2, int32(index)),
			name: "enum value declaration",
		})
	}
}

func reconstruct(
	byPath map[string]*descriptorpb.FileDescriptorProto,
) (*protoregistry.Files, map[string]protoreflect.FileDescriptor, error) {
	files := new(protoregistry.Files)
	filesByPath := make(map[string]protoreflect.FileDescriptor, len(byPath))
	pending := make(map[string]*descriptorpb.FileDescriptorProto, len(byPath))
	for path, file := range byPath {
		pending[path] = file
	}

	for len(pending) > 0 {
		paths := make([]string, 0, len(pending))
		for path := range pending {
			paths = append(paths, path)
		}
		sort.Strings(paths)

		builtOne := false
		for _, path := range paths {
			file := pending[path]
			if !dependenciesBuilt(file, filesByPath) {
				continue
			}
			descriptor, err := protodesc.NewFile(file, files)
			if err != nil {
				return nil, nil, malformed("reconstruct file %q: %w", path, err)
			}
			if err := files.RegisterFile(descriptor); err != nil {
				return nil, nil, malformed("register file %q: %w", path, err)
			}
			filesByPath[path] = descriptor
			delete(pending, path)
			builtOne = true
		}
		if builtOne {
			continue
		}

		paths = paths[:0]
		for path := range pending {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		return nil, nil, malformed("FileDescriptorSet has a cyclic import dependency involving %q", paths[0])
	}

	return files, filesByPath, nil
}

func dependenciesBuilt(
	file *descriptorpb.FileDescriptorProto,
	built map[string]protoreflect.FileDescriptor,
) bool {
	for _, dependency := range file.Dependency {
		if _, found := built[dependency]; !found {
			return false
		}
	}
	return true
}

func countFileDescriptors(file *descriptorpb.FileDescriptorProto, maxDepth int) (int, error) {
	count := len(file.GetService()) + len(file.GetEnumType()) + len(file.GetExtension())
	for _, service := range file.GetService() {
		if service == nil {
			return 0, malformed("file %q contains a nil service descriptor", file.GetName())
		}
		count += len(service.GetMethod())
	}
	for _, enum := range file.GetEnumType() {
		if enum == nil {
			return 0, malformed("file %q contains a nil enum descriptor", file.GetName())
		}
		count += len(enum.GetValue())
	}
	type messageFrame struct {
		message *descriptorpb.DescriptorProto
		depth   int
	}
	frames := make([]messageFrame, 0, len(file.GetMessageType()))
	for _, message := range file.GetMessageType() {
		frames = append(frames, messageFrame{message: message, depth: 1})
	}
	for len(frames) > 0 {
		last := len(frames) - 1
		frame := frames[last]
		frames = frames[:last]
		if frame.depth > maxDepth {
			return 0, overLimit("file %q contains a nested message deeper than the %d-level limit", file.GetName(), maxDepth)
		}
		message := frame.message
		if message == nil {
			return 0, malformed("file %q contains a nil message descriptor", file.GetName())
		}
		count += 1 + len(message.GetField()) + len(message.GetExtension()) + len(message.GetOneofDecl())
		for _, enum := range message.GetEnumType() {
			if enum == nil {
				return 0, malformed("file %q contains a nil nested enum descriptor", file.GetName())
			}
			count += 1 + len(enum.GetValue())
		}
		for _, nested := range message.GetNestedType() {
			frames = append(frames, messageFrame{message: nested, depth: frame.depth + 1})
		}
	}
	return count, nil
}

type extensionKey struct {
	extendee protoreflect.FullName
	number   protoreflect.FieldNumber
}

func extensionNames(files *protoregistry.Files) map[extensionKey]string {
	names := make(map[extensionKey]string)
	files.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		collectFileExtensions(file, names)
		return true
	})
	return names
}

func collectFileExtensions(file protoreflect.FileDescriptor, names map[extensionKey]string) {
	collectExtensions(file.Extensions(), names)
	messages := make([]protoreflect.MessageDescriptor, 0, file.Messages().Len())
	for i := range file.Messages().Len() {
		messages = append(messages, file.Messages().Get(i))
	}
	for len(messages) > 0 {
		last := len(messages) - 1
		message := messages[last]
		messages = messages[:last]
		collectExtensions(message.Extensions(), names)
		for i := range message.Messages().Len() {
			messages = append(messages, message.Messages().Get(i))
		}
	}
}

func collectExtensions(extensions protoreflect.ExtensionDescriptors, names map[extensionKey]string) {
	for i := range extensions.Len() {
		extension := extensions.Get(i)
		extendee := extension.ContainingMessage()
		if extendee == nil {
			continue
		}
		names[extensionKey{extendee: extendee.FullName(), number: extension.Number()}] = string(extension.FullName())
	}
}

func collectUnknownOptions(
	set *descriptorpb.FileDescriptorSet,
	locations *Locations,
	extensions map[extensionKey]string,
) []*manifestv1alpha1.UnknownOption {
	var unknown []*manifestv1alpha1.UnknownOption
	for _, file := range set.File {
		fileLocation := locations.FileOptions(file.GetName())
		unknown = append(unknown, unknownFromMessage(
			file.GetOptions(),
			manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_FILE,
			fileLocation,
			extensions,
		)...)
		for index, service := range file.Service {
			serviceLocation := locations.Path(file.GetName(), []int32{6, int32(index)})
			unknown = append(unknown, unknownFromMessage(
				service.GetOptions(),
				manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_SERVICE,
				serviceLocation,
				extensions,
			)...)
			for methodIndex, method := range service.Method {
				unknown = append(unknown, unknownFromMessage(
					method.GetOptions(),
					manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_METHOD,
					locations.Path(file.GetName(), []int32{6, int32(index), 2, int32(methodIndex)}),
					extensions,
				)...)
			}
		}
		for index, message := range file.MessageType {
			collectUnknownMessageOptions(
				file.GetName(),
				message,
				[]int32{4, int32(index)},
				locations,
				extensions,
				&unknown,
			)
		}
		for index, enum := range file.EnumType {
			collectUnknownEnumOptions(
				file.GetName(),
				enum,
				[]int32{5, int32(index)},
				locations,
				extensions,
				&unknown,
			)
		}
	}

	sort.Slice(unknown, func(i, j int) bool {
		if unknown[i].GetFullName() != unknown[j].GetFullName() {
			return unknown[i].GetFullName() < unknown[j].GetFullName()
		}
		if unknown[i].GetFieldNumber() != unknown[j].GetFieldNumber() {
			return unknown[i].GetFieldNumber() < unknown[j].GetFieldNumber()
		}
		if unknown[i].GetLocation().GetImportPath() != unknown[j].GetLocation().GetImportPath() {
			return unknown[i].GetLocation().GetImportPath() < unknown[j].GetLocation().GetImportPath()
		}
		if unknown[i].GetWireType() != unknown[j].GetWireType() {
			return unknown[i].GetWireType() < unknown[j].GetWireType()
		}
		if string(unknown[i].GetRawValue()) != string(unknown[j].GetRawValue()) {
			return string(unknown[i].GetRawValue()) < string(unknown[j].GetRawValue())
		}
		return string(unknown[i].GetEncodedField()) < string(unknown[j].GetEncodedField())
	})
	return unknown
}

func collectUnknownMessageOptions(
	filePath string,
	message *descriptorpb.DescriptorProto,
	path []int32,
	locations *Locations,
	extensions map[extensionKey]string,
	unknown *[]*manifestv1alpha1.UnknownOption,
) {
	type messageFrame struct {
		message *descriptorpb.DescriptorProto
		path    []int32
	}
	frames := []messageFrame{{message: message, path: path}}
	for len(frames) > 0 {
		last := len(frames) - 1
		frame := frames[last]
		frames = frames[:last]
		*unknown = append(*unknown, unknownFromMessage(
			frame.message.GetOptions(),
			manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_MESSAGE,
			locations.Path(filePath, frame.path),
			extensions,
		)...)
		for index, field := range frame.message.Field {
			*unknown = append(*unknown, unknownFromMessage(
				field.GetOptions(),
				manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_FIELD,
				locations.Path(filePath, appendPath(frame.path, 2, int32(index))),
				extensions,
			)...)
		}
		for index, oneof := range frame.message.OneofDecl {
			*unknown = append(*unknown, unknownFromMessage(
				oneof.GetOptions(),
				manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_ONEOF,
				locations.Path(filePath, appendPath(frame.path, 8, int32(index))),
				extensions,
			)...)
		}
		for index, enum := range frame.message.EnumType {
			collectUnknownEnumOptions(
				filePath,
				enum,
				appendPath(frame.path, 4, int32(index)),
				locations,
				extensions,
				unknown,
			)
		}
		for index, nested := range frame.message.NestedType {
			frames = append(frames, messageFrame{
				message: nested,
				path:    appendPath(frame.path, 3, int32(index)),
			})
		}
	}
}

func collectUnknownEnumOptions(
	filePath string,
	enum *descriptorpb.EnumDescriptorProto,
	path []int32,
	locations *Locations,
	extensions map[extensionKey]string,
	unknown *[]*manifestv1alpha1.UnknownOption,
) {
	*unknown = append(*unknown, unknownFromMessage(
		enum.GetOptions(),
		manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_ENUM,
		locations.Path(filePath, path),
		extensions,
	)...)
	for index, value := range enum.Value {
		*unknown = append(*unknown, unknownFromMessage(
			value.GetOptions(),
			manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_ENUM_VALUE,
			locations.Path(filePath, appendPath(path, 2, int32(index))),
			extensions,
		)...)
	}
}

func unknownFromMessage(
	message proto.Message,
	kind manifestv1alpha1.OptionExtendeeKind,
	location Location,
	extensions map[extensionKey]string,
) []*manifestv1alpha1.UnknownOption {
	if message == nil {
		return nil
	}
	raw := message.ProtoReflect().GetUnknown()
	var unknown []*manifestv1alpha1.UnknownOption
	for len(raw) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(raw)
		if tagLength < 0 {
			break
		}
		valueLength := protowire.ConsumeFieldValue(number, wireType, raw[tagLength:])
		if valueLength < 0 {
			break
		}
		value := append([]byte(nil), raw[tagLength:tagLength+valueLength]...)
		encodedField := append([]byte(nil), raw[:tagLength+valueLength]...)
		extendee := message.ProtoReflect().Descriptor().FullName()
		unknown = append(unknown, &manifestv1alpha1.UnknownOption{
			ExtendeeKind: kind,
			FieldNumber:  uint32(number),
			FullName:     extensions[extensionKey{extendee: extendee, number: number}],
			RawValue:     value,
			Location:     location.Manifest(),
			WireType:     uint32(wireType),
			EncodedField: encodedField,
		})
		raw = raw[tagLength+valueLength:]
	}
	return unknown
}

func appendPath(path []int32, values ...int32) []int32 {
	result := make([]int32, 0, len(path)+len(values))
	result = append(result, path...)
	return append(result, values...)
}
