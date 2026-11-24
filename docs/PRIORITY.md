# Priority Scoring Formula Specification

## 1. Overview
In disaster Search-and-Rescue (SAR) operations, automated detection streams must be triaged into actionable, unambiguous priorities for human incident commanders. The priority score $S \in [0.0, 1.0]$ ranks situations by immediate threat to human life and operational urgency.

---

## 2. Mathematical Formulation

For an entity or detection $e$ of class $k \in \{\text{person}, \text{fire}, \text{structural\_damage}, \text{flood}, \text{debris}\}$, the priority score is computed as:

$$S = \min\left(1.0, W_{\text{class}}(k) \cdot C + M + P + H_{\text{prox}}\right)$$

Where:

### 2.1 Class Severity Weight $W_{\text{class}}(k)$
Reflects operational urgency:
- $\text{person}$: $1.00$ — Saving human lives is the unconditional top objective.
- $\text{fire}$: $0.85$ — Rapidly spreading hazard endangering survivors and responders.
- $\text{structural\_damage}$: $0.75$ — Imminent structural collapse risk.
- $\text{flood}$: $0.65$ — Rising water / submersion hazard.
- $\text{debris}$: $0.45$ — Access blockage / route impediment.

### 2.2 Detection Confidence $C \in [0.0, 1.0]$
The classifier or detector probability score.

### 2.3 Cross-Modal Agreement Boost $M$
When multiple complementary sensors (RGB visible spectrum and Thermal infrared) independently verify an object, false positive rates drop drastically and confidence rises:
$$M = \begin{cases} +0.10 & \text{if modalities contain both } \text{rgb} \text{ and } \text{thermal} \\ 0.00 & \text{otherwise} \end{cases}$$

### 2.4 Persistence / Track Age $P$
Transient detector glitches on aerial video must not trigger emergency alerts; verified persistent entities confirmed across multiple frames gain persistence weight:
$$P = \min\left(1.0, \frac{N_{\text{obs}}}{N_{\text{persist}}}\right) \cdot W_{\text{persist}}$$
Default configuration: $N_{\text{persist}} = 5$ observations, $W_{\text{persist}} = 0.10$.

### 2.5 Hazard Proximity Boost $H_{\text{prox}}$
Survivors located in the immediate vicinity of active hazards face extreme peril. For a survivor ($\text{person}$) located at distance $d$ from the nearest active hazard within radius $R = 50.0\text{m}$:
$$H_{\text{prox}} = \max_{h \in \text{hazards}, d(e, h) \le R} \left( \beta_h \cdot \left(1.0 - \frac{d(e, h)}{R}\right) \right)$$
Hazard proximity coefficients $\beta_h$:
- $\text{fire}$: $0.25$
- $\text{flood}$: $0.20$
- $\text{structural\_damage}$: $0.15$
- $\text{debris}$: $0.10$

If no hazard is within $R$, $H_{\text{prox}} = 0.0$.
For hazard entities, $H_{\text{prox}} = 0.0$.

---

## 3. Priority Tiers & Action Thresholds

| Priority Score $S$ | Tier | Operational Meaning | Action |
|---|---|---|---|
| $0.90 \le S \le 1.00$ | **Critical** | Immediate life threat (e.g. survivor near fire/flood) | Instant emergency dispatcher alert |
| $0.80 \le S < 0.90$ | **High** | Confirmed survivor or active severe hazard | High-priority responder queue |
| $0.65 \le S < 0.80$ | **Medium** | Detected hazard or tentative survivor | Monitored situational awareness map |
| $S < 0.65$ | **Low** | Routine detection / debris | Persisted to map; no audible alert |

---

## 4. Ordering Invariants

The scoring implementation must guarantee the following axiomatic invariants:

1. **Hazard Proximity Invariant**: A survivor adjacent to an active hazard scores strictly higher than the identical survivor in isolation:
   $$S(\text{person}, C, \text{near\_hazard}) > S(\text{person}, C, \text{isolated})$$
2. **Modal Agreement Invariant**: Cross-modal confirmation (RGB + Thermal) scores strictly higher than single-modality at identical confidence:
   $$S(\text{modalities}=[\text{rgb}, \text{thermal}]) > S(\text{modalities}=[\text{rgb}])$$
3. **Class Severity Invariant**: Under identical confidence and persistence:
   $$S(\text{person}) > S(\text{fire}) > S(\text{structural\_damage}) > S(\text{flood}) > S(\text{debris})$$
4. **Confidence Monotonicity**: Score is strictly monotonic with respect to confidence:
   $$C_1 > C_2 \implies S(C_1) \ge S(C_2)$$
5. **Persistence Monotonicity**: An entity observed continuously across $N$ passes scores higher than a single isolated frame glimpse:
   $$S(N_{\text{obs}} \ge 5) > S(N_{\text{obs}} = 1)$$
6. **Range Boundary Invariant**:
   $$\forall e, \quad 0.0 \le S(e) \le 1.0$$
