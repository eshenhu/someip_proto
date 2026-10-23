# SOME/IP manifest: smart-workshop-lighting-sd-tp

## Provenance
- Manifest format: `1`
- Options API: `v1alpha1`
- Input FDS SHA-256: `08e8ba485f2f2143d4648abd5f445594634ca06ad59d81c4f91803a04ba65aea`
- Tool version: `v0.1.0`
- Source revision: `v0.1.0`
- Normative source commit: `94e4ea932e50273897a014bc57f7ae89f1816b84`

## Deployment profile
- Name: `smart-workshop-lighting-sd-tp`
- Version: `1.0.0`
- UDP application payload budget: `1400` bytes

### Provider identities
#### `smart-workshop-lighting-sd-tp`

- Service/instance ID: `18961` / `1`
- Lineage: `smart-workshop-lighting-sd-tp`
- Default endpoint: `provider-udp`
- Endpoints:
  - `provider-udp`: `TRANSPORT_PROTOCOL_UDP` `192.0.2.30` port `30500`
- Maximum application payload: `1502` bytes
- Maximum SOME/IP message: `1518` bytes
- Service-version bindings:
  - `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPService` (lineage `smart-workshop-lighting-sd-tp`, major `1`, maximum payload `1502` bytes)
    - Static Service Discovery offer:
      - Advertised endpoints: `provider-udp`
      - Service Discovery timing: live TTL `16777215` seconds
        - Initial delay: `0s 0ns..0s 10000000ns`
        - Repetitions: `3` every `0s 10000000ns`
        - Request-response delay: `0s 0ns..0s 5000000ns`
        - Cyclic delay: `1s 0ns`
    - Event-group delivery policies:
      - `multicast_previews`: `MULTICAST`; multicast 239.192.0.1 port 30502
      - `unicast_snapshots`: `UNICAST`
    - Provider TP declarations:
      - Member `1`: `tp_when_needed=true`
      - Member `2`: `tp_when_needed=false`
      - Member `32769`: `tp_when_needed=true`
      - Member `32770`: `tp_when_needed=true`
    - Derived provider TP analysis:
      - Member `1` (`tp_when_needed=true`):
        - `REQUEST`: payload `1` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `NEVER_NEEDED`; UDP budget `1400` bytes; runtime TP/session handling `false`
        - `RESPONSE`: payload `2` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `NEVER_NEEDED`; UDP budget `1400` bytes; runtime TP/session handling `false`
      - Member `2` (`tp_when_needed=false`):
        - `REQUEST`: payload `2` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`
        - `RESPONSE`: payload `2` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`
      - Member `32769` (`tp_when_needed=true`):
        - `NOTIFICATION`: payload `2..1502` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `MAY_BE_NEEDED`; UDP budget `1400` bytes; runtime TP/session handling `true`
      - Member `32770` (`tp_when_needed=true`):
        - `NOTIFICATION`: payload `1500` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `REQUIRED`; UDP budget `1400` bytes; runtime TP/session handling `true`

### Consumer bindings
#### `smart-workshop-lighting-dashboard`

- Target service: `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPService` (lineage `smart-workshop-lighting-sd-tp`, major `1`)
- Default local endpoint: `dashboard-udp`
- Provider selector: named provider `smart-workshop-lighting-sd-tp`
  - Candidates:
    - `smart-workshop-lighting-sd-tp` (service/instance `18961` / `1`, `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPService` major `1`, lineage `smart-workshop-lighting-sd-tp`)
- Local endpoints:
  - `dashboard-udp`: `TRANSPORT_PROTOCOL_UDP` `192.0.2.40` port `30510..30520`
- Service Discovery timing: live TTL `10` seconds
  - Initial delay: `0s 0ns..0s 10000000ns`
  - Repetitions: `1` every `0s 10000000ns`
  - Request-response delay: `0s 0ns..0s 5000000ns`
  - Cyclic delay: `1s 0ns`
- Subscriptions:
  - Event group `multicast_previews` (ID `2`, TTL `10` seconds, initial data `false`)
    - Local endpoints: none
    - Effective delivery policy: `MULTICAST`; multicast 239.192.0.1 port 30502
  - Event group `unicast_snapshots` (ID `1`, TTL `10` seconds, initial data `true`)
    - Local endpoints: `dashboard-udp`
    - Effective delivery policy: `UNICAST`
- Inherited provider TP analysis:
  - Member `1` (`tp_when_needed=true`):
    - `REQUEST`: payload `1` bytes; transport `UDP`; endpoint `dashboard-udp TRANSPORT_PROTOCOL_UDP 192.0.2.40 port 30510..30520`; result `NEVER_NEEDED`; UDP budget `1400` bytes; runtime TP/session handling `false`
    - `RESPONSE`: payload `2` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `NEVER_NEEDED`; UDP budget `1400` bytes; runtime TP/session handling `false`
  - Member `2` (`tp_when_needed=false`):
    - `REQUEST`: payload `2` bytes; transport `UDP`; endpoint `dashboard-udp TRANSPORT_PROTOCOL_UDP 192.0.2.40 port 30510..30520`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`
    - `RESPONSE`: payload `2` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `TP_NOT_CONFIGURED`; UDP budget `1400` bytes; runtime TP/session handling `false`
  - Member `32769` (`tp_when_needed=true`):
    - `NOTIFICATION`: payload `2..1502` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `MAY_BE_NEEDED`; UDP budget `1400` bytes; runtime TP/session handling `true`
  - Member `32770` (`tp_when_needed=true`):
    - `NOTIFICATION`: payload `1500` bytes; transport `UDP`; endpoint `provider-udp TRANSPORT_PROTOCOL_UDP 192.0.2.30 port 30500`; result `REQUIRED`; UDP budget `1400` bytes; runtime TP/session handling `true`

## Services
### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPService`

- Lineage: `smart-workshop-lighting-sd-tp`
- Interface version: `1.0`
- Event groups:
  - `1` `unicast_snapshots`
  - `2` `multicast_previews`
- Members:
  - `1` `GetLightingState` (`method`)
    - Method kind: `METHOD_KIND_REQUEST_RESPONSE`; result policy: `RESPONSE_POLICY_RESPONSE_ONLY`
    - Directional payload bounds:
      - `REQUEST`: `1` bytes
      - `RESPONSE`: `2` bytes
  - `2` `SetLightingState` (`method`)
    - Method kind: `METHOD_KIND_REQUEST_RESPONSE`; result policy: `RESPONSE_POLICY_RESPONSE_ONLY`
    - Directional payload bounds:
      - `REQUEST`: `2` bytes
      - `RESPONSE`: `2` bytes
  - `32769` `preview_updated` (`event`)
    - Event groups: `multicast_previews`
    - Publication: `PUBLICATION_POLICY_KIND_ON_CHANGE`
    - Directional payload bounds:
      - `NOTIFICATION`: `2..1502` bytes
  - `32770` `lighting_snapshot_notifier` (`field_notifier`)
    - Event groups: `unicast_snapshots`
    - Publication: `PUBLICATION_POLICY_KIND_ON_CHANGE`
    - Directional payload bounds:
      - `NOTIFICATION`: `1500` bytes
- Logical fields:
  - `lighting_snapshot` value `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSnapshot`
    - Notifier ID: `32770`
    - Event groups: `unicast_snapshots`
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

### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingPreviewUpdate`

- Encoded size: `2..1502` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `pixels` (wire order `1`, field `1`): `2..1502` bytes
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_16`

### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPServiceGetLightingStateRequest`

- Encoded size: `1` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `request_token` (wire order `1`, field `1`): `1` bytes
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`

### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPServiceGetLightingStateResponse`

- Encoded size: `2` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `state` (wire order `1`, field `1`): `2` bytes
    - Type: `.io.github.someip_proto.examples.v1alpha1.LightingState`
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`

### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPServiceSetLightingStateRequest`

- Encoded size: `2` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `desired_state` (wire order `1`, field `1`): `2` bytes
    - Type: `.io.github.someip_proto.examples.v1alpha1.LightingState`
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`

### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSDTPServiceSetLightingStateResponse`

- Encoded size: `2` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `accepted_state` (wire order `1`, field `1`): `2` bytes
    - Type: `.io.github.someip_proto.examples.v1alpha1.LightingState`
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_UNSPECIFIED`

### `.io.github.someip_proto.examples.v1alpha1.SmartWorkshopLightingSnapshot`

- Encoded size: `1500` bytes
- Byte order: `BYTE_ORDER_BIG_ENDIAN`
- Struct length width: `LENGTH_FIELD_WIDTH_NONE`
- Extensible: `false`
- Layout:
  - Member `pixels` (wire order `1`, field `1`): `1500` bytes
    - Effective byte order: `BYTE_ORDER_BIG_ENDIAN`; framing width: `LENGTH_FIELD_WIDTH_NONE`

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

