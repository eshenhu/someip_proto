# SOME/IP manifest: smart-workshop-lighting

## Provenance
- Manifest format: `1`
- Options API: `v1alpha1`
- Input FDS SHA-256: `ded0f71bba9a4328cb2c7f246bb2a0bd87a2c6f9abfd370eddc7ac470df2bbc4`
- Tool version: `v0.1.0`
- Source revision: `v0.1.0`
- Normative source commit: `94e4ea932e50273897a014bc57f7ae89f1816b84`

## Deployment profile
- Name: `smart-workshop-lighting`
- Version: `1.0.0`
- UDP application payload budget: `1400` bytes

### Provider identities
#### `smart-workshop-lighting`

- Service/instance ID: `18960` / `1`
- Lineage: `smart-workshop-lighting`
- Default endpoint: `primary`
- Endpoints:
  - `primary`: `TRANSPORT_PROTOCOL_UDP` `192.0.2.20` port `30500`
- Maximum application payload: `2` bytes
- Maximum SOME/IP message: `18` bytes
- Service-version bindings:
  - `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingService` (lineage `smart-workshop-lighting`, major `1`, maximum payload `2` bytes)
    - Static Service Discovery offer: none
    - Event-group delivery policies: none
    - Provider TP declarations:
      - Member `1`: `tp_when_needed=false`
      - Member `32769`: `tp_when_needed=false`
      - Member `32770`: `tp_when_needed=false`
    - Derived provider TP analysis:
      - Member `1` (`tp_when_needed=false`):
        - `REQUEST`: payload `2` bytes; transport `UDP`; endpoint `primary TRANSPORT_PROTOCOL_UDP 192.0.2.20 port 30500`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`
        - `RESPONSE`: payload `2` bytes; transport `UDP`; endpoint `primary TRANSPORT_PROTOCOL_UDP 192.0.2.20 port 30500`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`
      - Member `32769` (`tp_when_needed=false`):
        - `NOTIFICATION`: payload `2` bytes; transport `UDP`; endpoint `primary TRANSPORT_PROTOCOL_UDP 192.0.2.20 port 30500`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`
      - Member `32770` (`tp_when_needed=false`):
        - `NOTIFICATION`: payload `2` bytes; transport `UDP`; endpoint `primary TRANSPORT_PROTOCOL_UDP 192.0.2.20 port 30500`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`

### Consumer bindings

None.

## Services
### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingService`

- Lineage: `smart-workshop-lighting`
- Interface version: `1.0`
- Event groups:
  - `1` `lighting_updates`
- Members:
  - `1` `SetLightingState` (`method`)
    - Method kind: `METHOD_KIND_REQUEST_RESPONSE`; result policy: `RESPONSE_POLICY_RESPONSE_ONLY`
    - Directional payload bounds:
      - `REQUEST`: `2` bytes
      - `RESPONSE`: `2` bytes
  - `32769` `lighting_state_changed` (`event`)
    - Event groups: `lighting_updates`
    - Publication: `PUBLICATION_POLICY_KIND_ON_CHANGE`
    - Directional payload bounds:
      - `NOTIFICATION`: `2` bytes
  - `32770` `lighting_state_notifier` (`field_notifier`)
    - Event groups: `lighting_updates`
    - Publication: `PUBLICATION_POLICY_KIND_ON_CHANGE`
    - Directional payload bounds:
      - `NOTIFICATION`: `2` bytes
- Logical fields:
  - `lighting_state` value `.io.github.someip_proto.examples.v1alpha1.LightingState`
    - Notifier ID: `32770`
    - Event groups: `lighting_updates`
    - Publication: `PUBLICATION_POLICY_KIND_ON_CHANGE`

## Payload types
### `.io.github.someip_proto.examples.v1alpha1.LightingState`

- Encoded size: `2` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `brightness_percent` (wire order `1`, field `1`): `1` bytes
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`
  - Member `mode` (wire order `2`, field `2`): `1` bytes
    - Type: `.io.github.someip_proto.examples.v1alpha1.LightingMode`
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`

### `.io.github.someip_proto.examples.v1alpha1.SetLightingStateRequest`

- Encoded size: `2` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `desired_state` (wire order `1`, field `1`): `2` bytes
    - Type: `.io.github.someip_proto.examples.v1alpha1.LightingState`
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`

### `.io.github.someip_proto.examples.v1alpha1.SetLightingStateResponse`

- Encoded size: `2` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `accepted_state` (wire order `1`, field `1`): `2` bytes
    - Type: `.io.github.someip_proto.examples.v1alpha1.LightingState`
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`

## Enum types
### `.io.github.someip_proto.examples.v1alpha1.LightingMode`

- Wire width: `8` bits
- Closed: `false`
- Values:
  - `0` `LIGHTING_MODE_UNSPECIFIED` (non-sendable placeholder)
  - `1` `LIGHTING_MODE_WORK`
  - `2` `LIGHTING_MODE_INSPECTION`

## Preserved unknown options

None.

## Diagnostics

None.

