// Package descriptor loads and indexes the FileDescriptorSet boundary used by
// the semantic backend.
package descriptor

const (
	defaultMaxInputBytes         = 64 << 20
	defaultMaxFiles              = 10_000
	defaultMaxDescriptors        = 100_000
	defaultMaxDescriptorDepth    = 128
	defaultMaxSourceLocations    = 1_000_000
	defaultMaxSourcePathElements = 1_024
)

// Limits bounds untrusted FileDescriptorSet inputs.
type Limits struct {
	// MaxInputBytes is the largest accepted encoded FileDescriptorSet.
	MaxInputBytes int
	// MaxFiles is the largest accepted transitive file closure.
	MaxFiles int
	// MaxDescriptors is the largest accepted number of declared descriptors.
	MaxDescriptors int
	// MaxDescriptorDepth is the largest accepted nested message depth.
	MaxDescriptorDepth int
	// MaxSourceLocations is the largest accepted number of SourceCodeInfo locations.
	MaxSourceLocations int
	// MaxSourcePathElements is the largest accepted SourceCodeInfo path length.
	MaxSourcePathElements int
}

// DefaultLimits returns conservative limits suitable for CLI use.
func DefaultLimits() Limits {
	return Limits{
		MaxInputBytes:         defaultMaxInputBytes,
		MaxFiles:              defaultMaxFiles,
		MaxDescriptors:        defaultMaxDescriptors,
		MaxDescriptorDepth:    defaultMaxDescriptorDepth,
		MaxSourceLocations:    defaultMaxSourceLocations,
		MaxSourcePathElements: defaultMaxSourcePathElements,
	}
}

func (l Limits) normalized() Limits {
	defaults := DefaultLimits()
	if l.MaxInputBytes <= 0 {
		l.MaxInputBytes = defaults.MaxInputBytes
	}
	if l.MaxFiles <= 0 {
		l.MaxFiles = defaults.MaxFiles
	}
	if l.MaxDescriptors <= 0 {
		l.MaxDescriptors = defaults.MaxDescriptors
	}
	if l.MaxDescriptorDepth <= 0 {
		l.MaxDescriptorDepth = defaults.MaxDescriptorDepth
	}
	if l.MaxSourceLocations <= 0 {
		l.MaxSourceLocations = defaults.MaxSourceLocations
	}
	if l.MaxSourcePathElements <= 0 {
		l.MaxSourcePathElements = defaults.MaxSourcePathElements
	}
	return l
}

func (l Limits) protobufRecursionLimit() int {
	const (
		minimum = 128
		margin  = 64
		maximum = 1_000_000
	)
	if l.MaxDescriptorDepth < minimum {
		return minimum
	}
	if l.MaxDescriptorDepth > maximum-margin {
		return maximum
	}
	return l.MaxDescriptorDepth + margin
}
