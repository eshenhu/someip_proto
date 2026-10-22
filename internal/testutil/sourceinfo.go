// Package testutil provides descriptor fixtures shared by repository tests.
package testutil

import (
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// CompleteSourceCodeInfo adds synthetic, exact declaration locations to file.
// It is intended only for descriptor fixtures built from generated Go metadata,
// which does not retain source locations.
func CompleteSourceCodeInfo(file *descriptorpb.FileDescriptorProto) {
	if file == nil {
		return
	}
	if file.SourceCodeInfo == nil {
		file.SourceCodeInfo = &descriptorpb.SourceCodeInfo{}
	}
	known := make(map[string]struct{}, len(file.GetSourceCodeInfo().GetLocation()))
	for _, location := range file.GetSourceCodeInfo().GetLocation() {
		if location == nil {
			continue
		}
		if len(location.GetSpan()) == 0 {
			location.Span = []int32{0, 0, 0}
		}
		known[sourcePathKey(location.GetPath())] = struct{}{}
	}
	add := func(path []int32) {
		key := sourcePathKey(path)
		if _, found := known[key]; found {
			return
		}
		known[key] = struct{}{}
		file.SourceCodeInfo.Location = append(file.SourceCodeInfo.Location, &descriptorpb.SourceCodeInfo_Location{
			Path: append([]int32(nil), path...),
			Span: []int32{0, 0, 0},
		})
	}
	add(nil)
	for index, enum := range file.GetEnumType() {
		addEnumLocations(add, enum, []int32{5, int32(index)})
	}
	for index, service := range file.GetService() {
		servicePath := []int32{6, int32(index)}
		add(servicePath)
		if service == nil {
			continue
		}
		for methodIndex := range service.GetMethod() {
			add(appendPath(servicePath, 2, int32(methodIndex)))
		}
	}
	for index := range file.GetExtension() {
		add([]int32{7, int32(index)})
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
		add(frame.path)
		if frame.message == nil {
			continue
		}
		for index := range frame.message.GetField() {
			add(appendPath(frame.path, 2, int32(index)))
		}
		for index, enum := range frame.message.GetEnumType() {
			addEnumLocations(add, enum, appendPath(frame.path, 4, int32(index)))
		}
		for index := range frame.message.GetExtension() {
			add(appendPath(frame.path, 6, int32(index)))
		}
		for index := range frame.message.GetOneofDecl() {
			add(appendPath(frame.path, 8, int32(index)))
		}
		for index, nested := range frame.message.GetNestedType() {
			frames = append(frames, messageFrame{message: nested, path: appendPath(frame.path, 3, int32(index))})
		}
	}
}

func addEnumLocations(add func([]int32), enum *descriptorpb.EnumDescriptorProto, path []int32) {
	add(path)
	if enum == nil {
		return
	}
	for index := range enum.GetValue() {
		add(appendPath(path, 2, int32(index)))
	}
}

func appendPath(path []int32, values ...int32) []int32 {
	result := make([]int32, 0, len(path)+len(values))
	result = append(result, path...)
	return append(result, values...)
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
