# Future Roadmap / TODO List

These features are approved but deferred to post-MVP. The backend should be designed to support them eventually.

## A. Smart Cost & Budgeting

- **Household Prices:** Each household tracks its own price history for ingredients.
- **Receipt Parsing (AI):** Upload a photo of a receipt -> AI extracts items -> updates Household ingredient prices.
- **Estimator:** Show estimated cost for the Weekly Meal Plan based on stored prices.

## B. Pantry & Fridge Integration

- **Photo Analysis (AI):** Upload a photo of a fridge shelf or grocery bag -> AI identifies ingredients -> adds to "Pantry".
- **Cooking Filters:** "Cook from Fridge" filter.
  - Uses the **Include/Exclude** logic based on Ingredient Roles (ignore Essentials, focus on Primary).

## C. Nutritional Guardrails

- **Data:** Ingredients will store nutritional info (from DB or AI).
- **Warnings:** Warn user if Weekly Plan is missing key macros (e.g., "Low on Protein").
