# SmartWorkshopLighting example

`io.github.someip_proto.examples.v1alpha1` contains the non-normative
`SmartWorkshopLightingService` interface and its
`smart-workshop-lighting` provider-only deployment profile. It also contains
`SmartWorkshopLightingSDTPService` and its complete static
`smart-workshop-lighting-sd-tp` provider/consumer deployment profile.

The example demonstrates a fixed-layout request/response method, a typed
event, a named event group, enum metadata, a static UDP endpoint, and an
immutable service lineage. It intentionally does **not** imply an encoder,
decoder, transport implementation, runtime Service Discovery behavior, TP
segmentation/reassembly, or generated runtime bindings.

The separate SD/TP service demonstrates static provider offers, complete offer
and find timing, exact consumer selection, fixed and dynamic consumer endpoint
facts, unicast and multicast event-group delivery, subscriptions including
initial-data intent, and `tp_when_needed` analysis with derived
`NEVER_NEEDED`, `MAY_BE_NEEDED`, and `REQUIRED` outcomes. It remains a static
contract: it does not emit SD/TP traffic or prescribe runtime session behavior.

The reusable interface is declared in
[`smart_workshop_lighting.proto`](../proto/io/github/someip_proto/examples/v1alpha1/smart_workshop_lighting.proto).
The closed deployment profile is declared separately in
[`smart_workshop_lighting_deployment.proto`](../proto/io/github/someip_proto/examples/v1alpha1/smart_workshop_lighting_deployment.proto).
The complete static SD/TP service and profile are declared in
[`smart_workshop_lighting_sd_tp.proto`](../proto/io/github/someip_proto/examples/v1alpha1/smart_workshop_lighting_sd_tp.proto)
and
[`smart_workshop_lighting_sd_tp_deployment.proto`](../proto/io/github/someip_proto/examples/v1alpha1/smart_workshop_lighting_sd_tp_deployment.proto).

## Checked-in v0.1.0 contract artifacts

[`compatibility/baselines/v0.1.0/smart-workshop-lighting/`](../compatibility/baselines/v0.1.0/smart-workshop-lighting/)
contains the provider-only core compatibility suite. Its `sd-tp/` child holds
the complete static SD/TP suite. Each suite has an FDS, byte-for-byte golden
manifest/report renderings, and an independent `SHA256SUMS` file.

Regenerate the artifacts only as an intentional reviewed contract update:

```sh
bash ./scripts/generate-phase4-artifacts.sh
```

Verify that regeneration changes no bytes:

```sh
make phase4-artifact-check
```