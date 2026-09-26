# Ubiquitous Language & Domain Glossary

This glossary defines implementation-free domain terms used throughout the Interactive Buying Journey (IBJ) engine, SDK, and APIs.
*Citation: Summarized from IBJ spec (private), commit 6d473fa.*

| Term | Canonical Meaning | Must Not Mean |
| --- | --- | --- |
| **Buyer** | A person exploring, comparing, or purchasing products on a merchant's storefront. | A merchant, platform operator, or necessarily authenticated account. |
| **Visitor** | A buyer session interacting without an asserted persistent identity. | An automatically recognized cross-store or profiled individual. |
| **Merchant** | The commercial owner and operator of an individual storefront and its business policies. | An individual marketplace seller unless explicitly designated. |
| **Storefront** | The customer-facing web or mobile interface presenting product merchandise and checkout entry points. | The IBJ administrative dashboard or engine internals. |
| **Catalog Item** | Merchant-authored description of a sellable product family. | A purchasable variant or live seller offer. |
| **Variant** | A purchasable product configuration with a unique SKU and specific physical or functional attributes. | An abstract product family. |
| **Offer** | A specific seller's pricing and fulfillment terms for a variant at a single point in time. | A universal or immutable product price. |
| **Candidate** | An eligible variant or offer retrieved by the engine for further ranking and comparison. | A finalized, approved recommendation. |
| **Eligibility Rule** | A mandatory hard constraint (stock, shipping, budget, policy) that must hold before a candidate can be displayed. | A soft personalization preference or model ranker boost. |
| **Buyer Intent** | The current shopping goal derived from explicit buyer actions and permitted session signals. | A permanent sensitive trait or blanket consent grant. |
| **Preference** | Buyer-selected or permitted contextual constraint that can be viewed, modified, or cleared at any time. | An immutable or inferred identity trait. |
| **Journey** | The buyer's evolving progression through discovery, deliberation, and checkout intent. | A generated chat conversation transcript. |
| **Experience Plan** | A typed, schema-validated JSON specification determining which approved sections appear in named storefront slots. | Arbitrary HTML, executable scripts, or permission to alter checkout totals. |
| **Recommendation** | An approved, policy-validated variant candidate displayed within a specific slot with explainable provenance. | A probabilistic prediction guaranteed to fit every buyer. |
| **Plugin** | A versioned, capability-constrained extension altering a specific engine behavior under merchant policy. | An arbitrary shell script or unconstrained background process. |
| **Skill** | An agent-facing workflow instruction and evaluation package for a bounded task. | Runtime authorization or substitute for engine policy rules. |
| **Evidence** | A verified catalog fact, explicit buyer declaration, or permitted event record justifying an engine decision. | Generative language model narrative unsupported by data. |
| **Sponsored Placement** | A clearly disclosed, paid product allocation in a designated storefront slot. | Undisclosed organic relevance boost. |
| **Session** | Bounded context spanning a single visit interaction period. | Cross-merchant profiling or perpetual tracking. |
| **Tenant** | An isolated merchant workspace with separate catalogs, policies, credentials, and telemetry. | A pooled multi-store database or shared buyer graph. |
| **Control** | The ordinary storefront experience with zero IBJ adaptive modifications. | A degraded or intentionally impaired baseline. |
| **Reason Code** | A machine-readable token referencing a verified fact or constraint explaining why a section or item is shown. | An unverified LLM narrative or arbitrary explanation. |
| **Enrichment Record**| A merchant-reviewed offline proposal for an attribute or compatibility edge with provenance and confidence. | An auto-published live catalog fact or unverified model inference. |
