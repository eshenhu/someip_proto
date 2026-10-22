# Custom option extension-number registry

Project custom-option extensions use the permanent range `62000`–`62099`.
Numbers are assigned per extendee kind and must never be reused, including
following retirement. This registry is authoritative for the options API.

| Number | Extendee | Planned extension | Status |
| ---: | --- | --- | --- |
| 62000 | `google.protobuf.FileOptions` | `io.github.someip_proto.options.v1alpha1.file` | Allocated |
| 62001 | `google.protobuf.ServiceOptions` | `io.github.someip_proto.options.v1alpha1.service` | Allocated |
| 62002 | `google.protobuf.MethodOptions` | `io.github.someip_proto.options.v1alpha1.method` | Allocated |
| 62003 | `google.protobuf.MessageOptions` | `io.github.someip_proto.options.v1alpha1.message` | Allocated |
| 62004 | `google.protobuf.FieldOptions` | `io.github.someip_proto.options.v1alpha1.field` | Allocated |
| 62005 | `google.protobuf.OneofOptions` | `io.github.someip_proto.options.v1alpha1.oneof` | Allocated |
| 62006 | `google.protobuf.EnumOptions` | `io.github.someip_proto.options.v1alpha1.enum` | Allocated |
| 62007 | `google.protobuf.EnumValueOptions` | `io.github.someip_proto.options.v1alpha1.enum_value` | Allocated |

Assignments `62008`–`62099` are unallocated. A future assignment requires an
explicit reviewed update to this file and must use a currently unallocated
number. Retired entries remain recorded with status `Retired`; they are never
reassigned.

Static Service Discovery and SOME/IP-TP organization does not allocate an
additional extension. The stable `someip_options.proto` façade retains the
eight aggregate extensions above and publicly re-exports the split common,
deployment, SD, and TP option messages.
