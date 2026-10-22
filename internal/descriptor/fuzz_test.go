package descriptor

import (
	"testing"

	testv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/test/v1alpha1"
)

func FuzzLoad(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("not a descriptor set"))
	f.Add(fixtureDescriptorSet(f, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto))

	limits := Limits{
		MaxInputBytes:  1 << 20,
		MaxFiles:       1_000,
		MaxDescriptors: 10_000,
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > limits.MaxInputBytes {
			t.Skip()
		}
		set, err := Load(input, limits)
		if err != nil {
			return
		}
		if set == nil || set.Files == nil || set.Locations == nil || len(set.AllFiles()) == 0 {
			t.Fatalf("Load() success returned incomplete descriptor set: %#v", set)
		}
	})
}
