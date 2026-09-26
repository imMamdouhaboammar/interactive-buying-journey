# Contracts Provenance

The JSON Schemas and example payloads in this directory are vendored directly from the private IBJ specification repository under owner decision D-27.

## Upstream Provenance

- **Source Repository:** `imMamdouhaboammar/interactive-buying-journey-spec` (private)
- **Source Commit:** `6d473fab6f0fe9d8b5a8f121619748256c181827` (`6d473fa`)
- **Citation:** "IBJ spec (private), commit 6d473fa"
- **Vendor Policy:** Only `contracts/schemas/*.json` and `contracts/examples/*.json` are mirrored into this repository. No internal spec prose, research notes, private decision registers, or business plans are included.

## Cryptographic Checksums (SHA-256)

### Schemas (`contracts/schemas/`)

| File | SHA-256 Checksum |
| --- | --- |
| `catalog-batch.schema.json` | `f118a13e32904a12331e05b06bee6be57dc5bcf1b042385d4ae25d05705c4e40` |
| `compose-request.schema.json` | `d9dc453862e87e353c87c3a1f399b6309c5b1205bf6739b938404384b22b6545` |
| `decision-envelope.schema.json` | `81a4aa710d35fb29c710fcba3a127386360914d8f0882d4d14c7973d1333fe48` |
| `enrichment-record.schema.json` | `09a346d8ceb7c7c2f5af484f4aa48c276ed479001c57a588a4bf64bed4323e8f` |
| `enrichment-review.schema.json` | `317f179696bdba392cad31a98e5b2ea127051e4bf5113cda056bec45ce75110c` |
| `event-envelope.schema.json` | `2ec45f83c54a6dc15efc2d0a2df7a24008afc8bc8fdb0e6ed2d8178c4159380b` |
| `experience-plan.schema.json` | `ff1b99ca5256a0f8b94b9fcaa2d58cfd8a6024ab2d80cf8b3832339b4569dd6b` |
| `plugin-manifest.schema.json` | `97aeca529413b3fd60930f0a4a0b3562e7822451fff953365f0acb54d2330f64` |

### Examples (`contracts/examples/`)

| File | SHA-256 Checksum |
| --- | --- |
| `catalog-batch.json` | `cd3e3fb963cfbbf316f54b250af3ab2846a92c9d1547f42a57e289342d24c4e2` |
| `compose-request.json` | `1ac05e77a4788398a0af6a7ba59a7fdba4b1e3df92fac25a647058378632471a` |
| `decision-envelope.json` | `edb51b27f4b1992509cba0d902dc9206d79eae5c91d43c6182f0c086604daf83` |
| `enrichment-compatibility.json` | `e9984fdcd9ba04c41b659023b678e13bdbfcd310c892227a34218a6a794d0a38` |
| `enrichment-record.json` | `798ebb686ac5e492398f202876347d561db6be12c262f364c14fa2157ca54d0e` |
| `enrichment-review.json` | `ec9fa6fe917d50e84dff4d7c792336e15805cc944ab5d2225f233a445df07444` |
| `event-envelope.json` | `1947fab048f75b81ee583d6ac7248f767ba024fd105e6e40fa5a66daca901375` |
| `experience-plan.json` | `f49d8992b00d36131bae9a4bd3010d8d88fb493cd7e69f881c9ddb4dd22f4fd0` |

## Integrity Verification

CI runs `python tools/validate_contracts.py --check-provenance`, which re-calculates the SHA-256 hash of each file on disk and fails if any hash diverges from this file.
