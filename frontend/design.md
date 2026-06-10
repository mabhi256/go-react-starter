# XYZMed OPD EHR: Design & Build Spec (`design.md`)

> **Audience:** Claude Code. **Goal:** generate the React implementation of the nine OPD EHR mockups, all sharing one design system.
> **Visual source of truth:** the refined HTML mockups (`06-patient-queue-v2.html`, `05-org-admin-refined.html`) define the *correct* look. The other original `.html` files (`07`–`13`) define *content and layout* but **must be re-skinned to this system**; their colour usage is the "before," not the target.
> **Read this whole file before writing any component.** The colour rules in §4 are non-negotiable and are the single most important thing in this document.

---

## 1. How to use this document

1. Scaffold the project: `npm create vite@latest -- --template react-ts`, then run **`npx shadcn@latest init`** (choose CSS variables, Neutral base colour, `src/components/ui` path). This generates `globals.css`, updates `tailwind.config`, and wires the shadcn variable layer.
2. Build the **foundations**: merge the clinical token block into `globals.css` (§5), update `tailwind.config` (§5), and add any `@layer components` classes to `globals.css`. Do not start on screens until this is done.
3. Build screens (§9) by composing library components. A screen should contain almost no raw colour values or ad-hoc spacing; it pulls from tokens and components.
4. Run the **Definition of Done** checklist (§12) against every screen before considering it complete.
5. When a mockup and this spec disagree on colour, **this spec wins**. When they disagree on content/fields, the mockup wins.

---

## 2. Product overview

A multi-tenant **OPD (outpatient) clinic EHR**, FHIR R4-aligned, for an Indian clinical setting (₹, +91 phones, Indian names/states, registration numbers like `MCI/2020/KA-12345`).

**Roles** (drive which screens/actions are visible): `org_admin`, `doctor`, `nurse`, `receptionist`, `lab_tech`.

**Clinical coding systems** appear in specific searchable fields (see §10):
- **ICD-10**: diagnoses / `Condition.code`
- **SNOMED CT**: allergies, problems, procedures
- **LOINC**: lab tests, observations, diagnostic reports
- **RxNorm**: medications (use where e-prescribing exists)

**Patient state flow** (memorize, colour must reflect it, §4):
`Checked-In → Waiting → With Nurse (Triage) → Ready → In Consult → Done`
Reception owns Checked-In/Waiting; doctors own In Consult/Done; nurses own Triage.

---

## 3. Tech stack & conventions

| Concern | Decision |
|---|---|
| Framework | React 18 + **TypeScript** (strict) |
| Build | Vite |
| Component primitives | **shadcn/ui**: installs unstyled Radix-based components into `src/components/ui/`. Use for all standard controls (Button, Dialog, Input, Select, Tabs, Table, Badge, Card, DropdownMenu, Popover, Tooltip, Sheet, Checkbox, ScrollArea, Separator). Do **not** rebuild what shadcn provides; extend/theme it via the token system. |
| Styling | **Tailwind (configured, not CDN)** + shadcn's CSS-variable layer (§5). shadcn and the clinical token system share the same `:root` variable approach; they're wired together in `globals.css` (see §5). |
| Icons | `lucide-react`: shadcn uses it by default; replace all inline SVGs from the mockups with matching lucide icons |
| State | Local component state + lightweight context for `role`, `org`, `theme`. Data layer is out of scope; stub with typed mock data in `/mocks`. |
| Routing | `react-router`: one route per screen (§9) |
| Fonts | **Inter** (400/500/600/700) via `@fontsource/inter`. Inter is intentional: dense clinical data wants a neutral, legible workhorse, not a display face. |

### Suggested file structure
```
src/
  components/
    ui/           # shadcn/ui primitives (auto-generated, do not hand-edit)
                  # Button Dialog Input Select Tabs Table Badge Card
                  # DropdownMenu Popover Tooltip Sheet Checkbox ScrollArea Separator
    layout/       AppShell.tsx  Sidebar.tsx  TopBar.tsx
    shared/       StatusDot.tsx  Avatar.tsx  Chip.tsx  StatCard.tsx
                  Notice.tsx  LiveIndicator.tsx  SearchInput.tsx
    clinical/     QueueRow.tsx  DeptRow.tsx  CodedSearchInput.tsx
                  VitalInput.tsx  EhrSection.tsx  ResultRow.tsx
  screens/        OrgAdmin/  PatientQueue/  PatientManagement/  Appointments/
                  EncounterEhr/  VoiceToEhr/  DoctorWorklist/  ResultsInbox/  NurseVitals/
  mocks/          patients.ts  staff.ts  queue.ts  results.ts  ...
  lib/            cn.ts  types.ts  fhir.ts
  globals.css     # shadcn base + clinical token overrides (§5)
```

### Conventions
- Component files are `PascalCase.tsx`, one component per file, **named default export**.
- Props are an exported `interface XxxProps`. No `any`.
- Use the `cn()` helper that shadcn generates at `lib/utils.ts` (clsx + tailwind-merge).
- **No `localStorage`/`sessionStorage`** unless explicitly asked. Theme via CSS class on `<html>`.
- Accessibility: every interactive element is keyboard-reachable; status is never conveyed by colour alone (§11).
- **No raw Tailwind palette utilities** (`bg-violet-500`, `text-sky-600`, etc.) anywhere outside `globals.css`. Enforce with a CI grep or ESLint `no-restricted-syntax` rule targeting the pattern `(bg|text|border|ring)-(red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]`.

---

## 4. Design philosophy: colour discipline (NON-NEGOTIABLE)

The mockups originally failed because **colour was used decoratively and to label sequential workflow stages across ~14 hue families**. The fix, and the rule for all React work:

> **Colour is either (a) semantic (it encodes clinical/operational meaning) or (b) a single disciplined identity channel (avatars only). It is NEVER decorative.**

### Hard rules
1. **One neutral ramp (slate) carries ~90% of every screen**: text, borders, surfaces, most chips, most avatars, category tags.
2. **One brand colour (blue)**: primary actions and the single "active / in-progress" state. Nothing else.
3. **Exactly three semantic colours, reserved for meaning:**
   - **Red** = critical / urgent / **abnormal lab** / **allergy**
   - **Amber** = caution / overdue wait / out-of-range (non-critical)
   - **Green** = normal / done / stable
4. **Sequential stages are NEUTRAL.** A pipeline (Checked-In → … → Done) is a *flow*, not categories. Render the stages as identical neutral chips; convey order with position and `›` separators. Colour appears **only at the exception** (the bottleneck wait → amber; urgent → red; done → muted green; active consult → blue).
5. **Categories are neutral.** Role (Doctor/Nurse/Reception), visit type (OPD/Follow-up/Walk-in) → neutral chips. They are not statuses, so they earn no colour.
6. **Status is encoded by a left accent bar + one pill**, not by a full-background colour wash. Faint row tints (~3% alpha) are allowed for urgent/active rows; full pastel washes are not.
7. **Avatars are the only decorative-colour exception**: a low-chroma, deterministic *identity tint* per person, drawn from a fixed 5-tint set that is **kept clear of the red/amber/green/blue semantic regions** so it can never be misread as status.
8. **No gradients on banners/cards.** No violet/sky/teal/pink/fuchsia/orange/cyan/indigo/rose anywhere except the avatar identity tints.
9. When red appears, it must be **the loudest thing on the screen** (e.g., the urgent-patients banner). If red is everywhere, it means nothing.

> Litmus test before adding any colour: *"What clinical or operational fact does this colour state?"* If the answer is "none / it looks nicer / it tells items apart," use neutral.

---

## 5. Design tokens

shadcn generates a `globals.css` with its own `--background`, `--foreground`, `--primary` etc. variables in HSL format. **Extend that file** (do not replace it) by appending the clinical token block. Then override shadcn's semantic variables to point at the clinical values, so that shadcn components (Button, Badge, etc.) automatically adopt the right colours without per-component overrides.

### `globals.css` additions (append after the shadcn-generated `:root` block)
```css
@layer base {
  :root {
    /* ── Clinical design tokens ──────────────────────────────── */
    /* Neutral ramp */
    --ink:       #0F172A; --ink-2: #334155; --ink-3: #64748B; --ink-4: #94A3B8;
    --line:      #E2E8F0; --line-soft: #F1F5F9;
    --surface:   #FFFFFF; --bg: #F8FAFC; --bg-2: #F1F5F9;

    /* Brand */
    --brand:       #2563EB; --brand-press: #1D4ED8; --brand-tint: #EFF6FF;

    /* Semantic (reserved for meaning, see §4) */
    --danger:      #DC2626; --danger-tint: #FEF2F2;
    --warn:        #B45309; --warn-tint:   #FFFBEB;
    --ok:          #15803D; --ok-tint:     #F0FDF4;

    /* Type scale: 6 steps only */
    --t-cap: 12px; --t-body: 13px; --t-emph: 14px;
    --t-sec: 16px; --t-num: 24px;  --t-title: 24px;

    /* Radius: 3 values only */
    --r-sm: 6px; --r-md: 10px; --r-full: 999px;

    /* Spacing: 4px grid */
    --s1:4px; --s2:8px; --s3:12px; --s4:16px; --s5:20px; --s6:24px;

    /* Elevation */
    --shadow:    0 1px 2px rgba(15,23,42,.04), 0 1px 3px rgba(15,23,42,.06);
    --shadow-lg: 0 16px 48px rgba(15,23,42,.16);

    /* Identity channel: avatars only, low-chroma, never status */
    --id-1-bg:#E7E5F0; --id-1-fg:#5B5277;
    --id-2-bg:#DCEAE6; --id-2-fg:#3D6B5E;
    --id-3-bg:#E6E3DA; --id-3-fg:#6B6451;
    --id-4-bg:#E2E6EC; --id-4-fg:#4A5568;
    --id-5-bg:#E9E2E6; --id-5-fg:#6E5560;

    /* ── Wire clinical values into shadcn semantic vars ─────── */
    /* shadcn reads these to theme its own Button, Badge, Input, etc. */
    --background:   var(--bg);
    --foreground:   var(--ink);
    --card:         var(--surface);
    --card-foreground: var(--ink);
    --popover:      var(--surface);
    --popover-foreground: var(--ink);
    --primary:      var(--brand);
    --primary-foreground: #ffffff;
    --secondary:    var(--bg-2);
    --secondary-foreground: var(--ink-2);
    --muted:        var(--bg-2);
    --muted-foreground: var(--ink-3);
    --accent:       var(--brand-tint);
    --accent-foreground: var(--brand);
    --destructive:  var(--danger);
    --destructive-foreground: #ffffff;
    --border:       var(--line);
    --input:        var(--line);
    --ring:         var(--brand);
    --radius:       var(--r-sm);   /* shadcn's --radius drives its own rounded-* scale */
  }
}
```

> **Note:** shadcn expects HSL values for some vars (`hsl(var(--primary))` syntax). If you run into HSL conflicts after `shadcn init`, switch the shadcn-generated vars to the hex values above and remove the `hsl()` wrappers from `tailwind.config`; this is cleaner for a custom design system than fighting the HSL convention.

### `tailwind.config.ts`: extend with semantic aliases
```ts
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink:      { DEFAULT: "var(--ink)", 2: "var(--ink-2)", 3: "var(--ink-3)", 4: "var(--ink-4)" },
        line:     { DEFAULT: "var(--line)", soft: "var(--line-soft)" },
        surface:  "var(--surface)",
        bg:       { DEFAULT: "var(--bg)", 2: "var(--bg-2)" },
        brand:    { DEFAULT: "var(--brand)", press: "var(--brand-press)", tint: "var(--brand-tint)" },
        danger:   { DEFAULT: "var(--danger)", tint: "var(--danger-tint)" },
        warn:     { DEFAULT: "var(--warn)",   tint: "var(--warn-tint)" },
        ok:       { DEFAULT: "var(--ok)",     tint: "var(--ok-tint)" },
      },
      borderRadius: { sm: "var(--r-sm)", md: "var(--r-md)", full: "var(--r-full)" },
      fontFamily:   { sans: ["Inter", "sans-serif"] },
      boxShadow:    { card: "var(--shadow)", lg: "var(--shadow-lg)" },
      fontSize: {
        cap: "var(--t-cap)", body: "var(--t-body)", emph: "var(--t-emph)",
        sec: "var(--t-sec)", num: "var(--t-num)",
      },
    },
  },
};
```
Devs reach for `bg-surface text-ink border-line` etc. **There must be no raw Tailwind palette utilities** (`bg-violet-500`, `text-sky-600`, …) anywhere in screen or component code; see Conventions (§3).

---

## 6. Foundations

**Typography**: 6 sizes only. Body default `--t-body` (13px). `--t-emph` (14px) for names/emphasis. `--t-sec` (16px) section titles. `--t-num`/`--t-title` (24px) metrics + page titles. Captions/meta `--t-cap` (12px). Weights 400/500/600/700. Page titles 19px / 700 with `letter-spacing:-.015em`. **Never** introduce a 7th size or a `.5px` value.

**Spacing**: multiples of 4px (`--s1`…`--s6`). Card padding 14–16px. Page padding 20–24px.

**Radius**: `--r-sm` (6px) controls/badges/chips · `--r-md` (10px) cards/modals · `--r-full` pills/dots/round avatars. No other radii.

**Borders**: always **1px** `--line`. No 1.5px. Hairline dividers use `--line-soft`.

**Elevation**: `--shadow` for cards, `--shadow-lg` for modals. No coloured shadows.

**Icons**: lucide-react, `w-4 h-4` (16px) default, `w-3.5 h-3.5` in dense rows, stroke width 2, `currentColor`.

---

## 7. Component library

Components fall into three tiers:

| Tier | Where | Rule |
|---|---|---|
| **shadcn/ui primitives** | `src/components/ui/` (auto-generated) | Use as-is or with variant extension via `cva`. Never hand-edit the generated file; re-run `npx shadcn@latest add <component>` to update. |
| **Extended shadcn** | Still in `ui/`, but with custom variants added | E.g. `Badge` gets clinical variants (urgent/waiting/active/done) added via `cva` on top of the shadcn base. |
| **Custom clinical** | `src/components/shared/` and `src/components/clinical/` | Built from scratch using shadcn primitives + the token system. No shadcn equivalent exists for these. |

### shadcn components to install
```bash
npx shadcn@latest add button dialog input select tabs table badge card \
  dropdown-menu popover tooltip sheet checkbox scroll-area separator avatar
```
After installing, extend `Badge` and `Button` with the clinical variants below. Do not re-theme them by overriding the generated file; use `cva` composition or a thin wrapper component.

### Button *(shadcn: extend with `tone` prop)*
```ts
// Wrap the shadcn Button to add a danger tone
interface ButtonProps extends React.ComponentProps<typeof ShadcnButton> {
  tone?: "default" | "danger";
}
// tone="danger": outline style, text-danger border-danger/40, hover:bg-danger-tint
// Never a solid red fill except a confirmed-destroy action inside a Modal
```

### Badge *(shadcn: extend with clinical variants via cva)*
```ts
// Add these variants to the shadcn Badge via cva or a wrapper:
type BadgeVariant = "neutral" | "urgent" | "waiting" | "active" | "done";
// neutral  → bg-bg-2 text-ink-3 border border-line
// urgent   → bg-danger-tint text-danger border border-danger/30
// waiting  → bg-warn-tint text-warn border border-warn/30       (overdue only)
// active   → bg-brand-tint text-brand border border-brand/30
// done     → bg-ok-tint text-ok border border-ok/30
// All use rounded-full (pill) and font-size cap (12px)
```
> **Rule:** category tags (OPD, Follow-up, Walk-in, Doctor, Nurse) always use `neutral`. The coloured variants are reserved for operational/clinical status.

### Dialog / Modal *(shadcn `Dialog`: use directly)*
shadcn `Dialog` with `DialogHeader`, `DialogFooter`, `DialogContent`. No customisation needed beyond wiring the token-driven colours (which come from the `globals.css` overrides in §5). For the Check-In modal's internal tab strip, use the `Tabs` component (below) inside `DialogContent`.

### Tabs *(shadcn: use directly)*
Use `variant="default"` (segment/pill) for page-level tabs (queue filter) and configure the underline style via `className` override for modal-internal tabs (check-in, encounter sections). Both pull colour from `--primary` / `--muted` which map to the clinical tokens.

### Input / Select *(shadcn: use directly)*
Focus ring maps to `--ring` → `--brand`. Labels via a `<Label>` component (shadcn). 11px uppercase `--ink-3` label style applied via `className`.

### Table *(shadcn: use directly)*
`Table`, `TableHeader`, `TableRow`, `TableHead`, `TableCell`. Apply `hover:bg-bg` on rows. Status cells use `Badge`; abnormal values use `className="text-danger font-semibold"`, out-of-range `"text-warn font-semibold"`.

### Card *(shadcn: use directly)*
Maps to `--card` / `--border` which resolve to `--surface` / `--line`. `CardHeader` + `CardContent` for panelled sections.

---
*The following are custom components (no shadcn equivalent):*

### StatusDot *(custom)* *(custom)*
```ts
interface StatusDotProps { tone: "neutral" | "brand" | "ok" | "warn" | "danger"; pulse?: boolean; }
```
7px round. `pulse` for live indicators (green). Map: in-consult→brand, available/normal→ok, waiting-backlog→warn, urgent→danger, idle/offline→neutral.

### Avatar *(custom: identity tint channel)*
```ts
interface AvatarProps {
  initials: string;
  id: 1 | 2 | 3 | 4 | 5;        // deterministic: hash(patientId|staffId) % 5 + 1
  shape?: "rounded" | "circle";  // rounded (r-sm) for patients, circle for staff
  size?: number;                 // px, default 38
}
```
Wraps the shadcn `Avatar` primitive but ignores its default fallback colour. Background/foreground from `--id-{n}-bg/fg`. **This is the only place non-semantic colour is allowed.** Export a `hashToId(id: string): 1|2|3|4|5` util; stable tint per person.

### Chip *(custom: pipeline stage count)*
```ts
interface ChipProps { variant?: "neutral" | "muted" | "warn"; children: ReactNode; }
```
Square-ish (`--r-sm`). `neutral` default, `muted` for zero/empty stages, `warn` **only** for over-threshold waits. Render stage flows as `<Chip /> <span>›</span> <Chip />` sequences.

### StatCard *(custom)*
```ts
interface StatCardProps { label: string; value: string; sub?: string; dot?: StatusDotProps["tone"]; }
```
Neutral surface (shadcn `Card`), value in `--ink` at `--t-num`. Optional leading `StatusDot` keys the metric's meaning: Waiting metric gets `warn`, Done gets `ok`, most get `neutral`.

### QueueRow / DeptRow *(custom: the status pattern)*
```ts
interface QueueRowProps {
  state: "waiting" | "active" | "done" | "urgent";
  overdue?: boolean;  // amber wait time + waiting Badge goes amber
  token: string; patient: Patient; meta: string; timeLabel: string; waitLabel?: string;
  actions?: ReactNode;
}
```
- Left **accent bar** (3px `::before`): transparent default → `--danger` urgent → `--brand` active. `done` → 62% opacity row.
- Faint full-row tint (~3% alpha) for urgent/active only. **Never** a full pastel wash.
- Status = accent bar + one `Badge`. Everything else neutral.
- `DeptRow` is the same pattern with `warn`/`alert` accent + a row of `Chip`s for stage counts.

### CodedSearchInput *(custom clinical)*
```ts
interface CodedSearchInputProps {
  system: "ICD-10" | "SNOMED" | "LOINC" | "RxNorm";
  placeholder?: string;
  onSelect: (code: { code: string; display: string; system: string }) => void;
}
```
Built on shadcn `Popover` + `Command`. Typeahead showing `code - display` rows with a small neutral system tag. Selected codes render as neutral `Chip`s with a remove affordance. See §10 for which field uses which system.

### Notice *(custom)*
```ts
interface NoticeProps { tone?: "info" | "danger"; icon?: LucideIcon; children: ReactNode; action?: ReactNode; }
```
Flat surface, **3px left accent** (`--brand` for info, `--danger` for danger). `danger` uses `--danger-tint` bg. Reserved for genuine operational alerts (urgent-patients banner, role-boundary reminders). **No gradients.**

### Layout: AppShell / Sidebar / TopBar / LiveIndicator *(custom)*
- `Sidebar`: dark `--ink` bg, 220px, grouped nav sections, active item `bg-white/10 text-white`, role-aware links, org switcher, user footer with circle `Avatar`.
- `TopBar`: white, 1px `--line` bottom, 19px page title, optional `LiveIndicator` (neutral pill + pulsing ok dot), right-aligned actions.
- `AppShell`: composes Sidebar + scrollable main column; TopBar sticky.

---

## 8. Interaction & non-visual states

Every list/table/screen must define: **loading** (skeletons in neutral `--bg-2`, no spinners-as-decoration), **empty** (neutral illustration-free message + primary action), **error** (Notice `tone="danger"`). Destructive actions confirm via Modal. Keep micro-animations subtle (150ms transitions; the only looping animation is the live-dot pulse).

---

## 9. Screen specs

Each screen = one route under `AppShell`. "Re-skin" means: keep the layout & content from the original mockup, replace all colour with the system per §4.

### 9.1 Org Admin: `/admin` · `OrgAdmin/`
Source: **`05-org-admin-refined.html` (already correct, match it).** Roles: `org_admin`.
Two views toggled in the sidebar: **Live Dashboard** and **Staff**.
- Dashboard: 6 neutral pipeline `StatCard`s (only Waiting=amber dot, In-Consult=brand dot, Done=ok dot); a red `Notice` for urgent patients; `DeptRow` list (neutral stage Chips, amber only at bottleneck, left accent warn/alert); "Waiting > 30 min" `Table`; throughput stats; right rail of Doctors/Nurses panels (StatusDot per availability) + Recent Activity (dot tone per event type).
- Staff: 5 summary StatCards (only Pending=amber); searchable/filterable staff `Table` (neutral role Badges; status = ok/neutral/warn; identity-tinted Avatars); Add-Staff `Modal` (role-conditional Department/Reg-No fields).
FHIR: `Organization`, `Practitioner`, `PractitionerRole`. 

### 9.2 Patient Queue (Reception): `/queue` · `PatientQueue/`
Source: **`06-patient-queue-v2.html` (already correct, match it).** Roles: `receptionist`.
TopBar with LiveIndicator + "Check In Patient" → **Check-In Modal** (3 internal tabs: *Scheduled* / *Search Patient* / *Register New*; urgent appt uses red left accent). Role-boundary `Notice` (info). 4 StatCards. Segment Tabs (All/Waiting/With Doctor/Done). `QueueRow` list using the accent-bar pattern; Reassign-Doctor `Modal`.
FHIR: `Encounter` (status), `Appointment`, `Patient`.

### 9.3 Patient Management: `/patients` · `PatientManagement/`
Source: `07-patient-management.html` (**re-skin**). Roles: `receptionist`, `org_admin`, `doctor` (read).
Master-detail: searchable patient `Table`/list → patient detail with **Demographics**, contact, visit history. Modals: **Register New Patient**, **Edit Patient**, **New Encounter**. Use identity-tinted Avatars; neutral status; demographics in a clean two-column `Card`.
FHIR: `Patient`, `Encounter`.

### 9.4 Appointments: `/appointments` · `Appointments/`
Source: `08-appointments.html` (**re-skin**). Roles: `receptionist`, `doctor`.
Calendar/day-agenda split into **Morning (09:00–13:00)** / **Afternoon (14:00–18:00)** time bands; week strip (Mon–Sun); per-doctor agenda. **New Appointment** Modal (Patient*, Doctor*, Date*, Time*, Type*, Reason/Chief complaint, Notes; `teleconsult` type supported). Appointment blocks: neutral by default; `urgent`→red, `teleconsult`→neutral with a small icon (not a colour). Avoid colour-coding doctors; use the identity tint on their avatar instead.
FHIR: `Appointment`, `Slot`, `Schedule`, `Practitioner`.

### 9.5 Encounter / EHR: `/encounter/:id` · `EncounterEhr/`  ⚠ highest stakes
Source: `09-encounter-ehr.html` (**re-skin: this file had 47 colours; reduce to the system**). Roles: `doctor`.
Three-column: left = section nav (`EhrSection` list: Chief Complaint, HPI, PMH, Surgical/Family/Social History, Assessment & Key Actions, Lab Tests, Procedures & Imaging, Referrals, Follow-Up) ; center = SOAP-style note editor; right = patient context (allergies, vitals, problems, meds).
**Clinical-safety colour mapping (critical):**
- **Allergies → red** (`AllergyIntolerance`, SNOMED). Always visible, always red.
- **Abnormal vitals/labs → red; out-of-range non-critical → amber; normal → neutral/green.**
- Diagnoses use `CodedSearchInput system="ICD-10"`; allergies & problems `SNOMED`; lab orders `LOINC`; meds `RxNorm`.
- Section accent dots from the original (violet/orange) → **neutral**. The numbered section markers stay neutral.
- "Encounter Finalised" = a single `Notice tone="info"` or a done Badge, not a celebratory colour block.
FHIR: `Encounter`, `Condition`, `AllergyIntolerance`, `Observation`, `MedicationRequest`, `ServiceRequest`, `Procedure`.

### 9.6 Voice-to-EHR: `/encounter/:id/voice` · `VoiceToEhr/`
Source: `10-tts-ehr.html` (**re-skin: remove the fuchsia**). Roles: `doctor`. State: **"Coming Soon."**
Recorder (timer `00:00`, neutral controls), live **Transcription** panel, **AI Field Extraction** mapping transcript → SOAP/HPI + suggested ICD-10/LOINC codes (Whisper ASR + Claude). The "Coming Soon" treatment = a neutral overlay/badge, **not** an accent hue. Extraction confidence shown via neutral text or a single brand accent, never a rainbow.
FHIR: writes back to `Encounter`/`Observation`/`Condition` (same as 9.5).

### 9.7 Doctor Worklist: `/worklist` · `DoctorWorklist/`
Source: `11-doctor-worklist.html` (**re-skin**). Roles: `doctor`.
"My Worklist" with stat tiles **Waiting for Me / In Consult / Seen Today**; "My Patients Today" list. **Start Encounter** Modal (Encounter Class, optional Chief Complaint); this is where `Encounter` is created (POST). In-Consult state = brand (active); waiting overdue = amber; done = muted green.
FHIR: `Encounter`, `Task`/worklist, `Patient`.

### 9.8 Results Inbox: `/results` · `ResultsInbox/`
Source: `12-results-inbox.html` (**re-skin**). Roles: `doctor`, `lab_tech`.
Grouped result queues: **Critical / Unreviewed**, **Abnormal / Unreviewed**, **Normal / Unreviewed**, **Acknowledged (7 days)**. `ResultRow` shows value + units + reference range; **critical→red, abnormal→amber, normal→neutral/green** (LOINC). Detail panel: **Acknowledge Result** Modal (Action Taken, Clinical Note, Add to Encounter). Critical results are the loudest things on the screen.
FHIR: `DiagnosticReport`, `Observation` (LOINC), `Encounter` (link).

### 9.9 Nurse: Triage & Vitals: `/triage` · `NurseVitals/`
Source: `13-nurse-vitals.html` (**re-skin: remove violet/teal/cyan**). Roles: `nurse`.
Patient header + **vitals capture** form via `VitalInput` (BP, HR, Temp, SpO2, RR, Height/Weight→auto **BMI**). Each vital validates against a reference range: in-range neutral, out-of-range amber, critical red. Triage notes; "mark Ready for Doctor" action (advances state).
FHIR: `Observation` (vital-signs LOINC panel), `Encounter`.

---

## 10. Coded-field reference (where each system is used)

| Field | Screen | `CodedSearchInput system` |
|---|---|---|
| Diagnosis (primary/secondary/differential) | Encounter EHR | `ICD-10` |
| Allergy / intolerance | Encounter EHR, patient context | `SNOMED` |
| Problem list | Encounter EHR | `SNOMED` (or ICD-10) |
| Procedure / imaging order | Encounter EHR | `SNOMED` |
| Lab test order | Encounter EHR | `LOINC` |
| Lab result type | Results Inbox | `LOINC` |
| Vital signs | Nurse Vitals | `LOINC` (vital-signs panel) |
| Medication | Encounter EHR (e-Rx) | `RxNorm` |

Each selected code renders as a **neutral chip** showing `display` with the `code` available on hover/secondary line. Never colour-code by coding system.

---

## 11. Accessibility & clinical safety

- **Colour is never the sole signal.** Every red/amber/green state also carries text and/or an icon (e.g. "Abnormal ↑", "Urgent"). A colour-blind clinician must get the same information.
- Target **WCAG AA** contrast. The semantic foregrounds (`--danger/#DC2626`, `--warn/#B45309`, `--ok/#15803D`) meet AA on white/tinted backgrounds; do not lighten them for "prettiness."
- Allergies and critical results must be visible without interaction (no hover-only reveal).
- Full keyboard navigation; visible focus ring (`--brand`); `aria-label`s on icon-only buttons; modals trap focus and close on Esc.
- Hit targets ≥ 32px in dense tables, ≥ 40px elsewhere.

---

## 12. Definition of Done (run per screen)

- [ ] Composed from library components; **zero** raw hex values and **zero** Tailwind palette utilities (`*-violet-*`, `*-sky-*`, etc.) in the file.
- [ ] Only neutral + brand + the 3 semantics + avatar identity tints appear. Grep proves it.
- [ ] Sequential stages render neutral; colour appears only at exceptions.
- [ ] Categories (role, visit type) are neutral; statuses are semantic.
- [ ] Status shown via left accent + single pill, not full-background washes.
- [ ] Type uses only the 6-step scale; radius only `sm/md/full`; borders 1px.
- [ ] Coded fields use `CodedSearchInput` with the correct system (§10).
- [ ] Allergies/critical results are red and always visible; abnormal=amber; normal=neutral/green.
- [ ] Colour is never the only signal (text/icon present); AA contrast; keyboard + focus states.
- [ ] Loading / empty / error states implemented.
- [ ] Matches the refined mockups (`05`, `06`) in feel; matches originals (`07`–`13`) in content but **not** their colour.

---

*End of spec. Build foundations → components → screens, in that order, and re-check §4 whenever you reach for a colour.*
