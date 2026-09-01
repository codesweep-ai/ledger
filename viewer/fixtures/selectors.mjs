// Selector map for the viewer fixture suite (viewer/fixtures/run.mjs).
//
// This is the ONLY file a viewer developer edits when the DOM changes. Every
// entry names a *behavioural role* in the page; the value says where that role
// lives in today's DOM. Expectation values in expectations.json are frozen
// against the roles, never against these strings.
//
// Entry forms:
//   "css"                              plain CSS selector (querySelectorAll)
//   { css: "…", text: "…" }            first element matching css whose trimmed
//                                      text equals `text` (case-insensitive)
//   { css: "…", textPrefix: "…" }      …whose trimmed text starts with `textPrefix`
//
// Keep every role resolvable: the runner fails a check whose role resolves to
// nothing when the check needs it, and names the role in the table.

export const selectors = {
  // ---- page chrome -------------------------------------------------------
  // Container that holds the current view's body (cards, lanes, rows, brief).
  viewBody: "#groups",
  // The view switcher buttons (brief / board / list / activity); matched by text.
  viewSwitch: ".viewbar [data-component='SegmentedControl'] [data-segmented-option], .viewbar > [data-component='Chip']",
  // The filter toolbar shown on board / list / activity (absent on brief).
  toolbar: ".controls.filterbar",
  // Every control in the filter toolbar whose label the suite records.
  toolbarControls: ".controls.filterbar [data-component='Chip'], .controls.filterbar [data-component='SearchInput'] [data-search-input]",
  // The free-text search box rendered inside SearchInput (documented data-search-input hook).
  search: ".controls.filterbar [data-component='SearchInput'] [data-search-input]",
  // Status filter cycler (all → open → in-progress → closed → ideas → retired → drafts).
  statusCycle: "#cycStatus",
  // Severity filter cycler (all → crit → high+ → med+). No id today: matched by label prefix.
  severityCycle: { css: ".controls.filterbar [data-component='Chip']", textPrefix: "sev" },
  // Type filter cycler (all → defects → improv → ideas).
  typeCycle: { css: ".controls.filterbar [data-component='Chip']", textPrefix: "type" },
  // Sort cycler, list view only (id → activity → severity → id).
  sortCycle: "#cycSort",
  // Group-by cycler, list view only (status → stint → type).
  groupCycle: "#cycGroup",
  // "reset" clears every filter; disabled when nothing is active.
  resetButton: { css: ".controls.filterbar [data-component='Chip']", text: "reset" },
  // The theme toggle rendered by @codesweep-ai/ui's ThemeToggle (cycles light → dark → system).
  themeToggle: "[data-component='ThemeToggle']",

  // ---- records -----------------------------------------------------------
  // One record card (board lanes, list groups, the brief's "up next"). Click opens the detail.
  card: ".bcard",
  // Attribute on `card` carrying the record key (issue id, or draft:<member>/<slug>).
  cardKeyAttr: "data-record",
  // The record id text inside a card, an activity row or the detail head.
  recordId: ".idm",
  // One activity-view row (click opens the record). Also used by the brief's "needs you".
  activityRow: ".actrow",
  // Severity badge text (crit / high / med / low).
  severityLabel: ".sev",
  // Type badge text (defect / improvement / idea).
  typeLabel: ".tag",
  // Status words visible on records: lane/group header titles, "in progress" marks, activity kinds.
  statusLabel: ".lane-header > span:first-child, .grp > header > span:first-child, .ip, .actk",

  // ---- board -------------------------------------------------------------
  // One board lane (Open / Drafts / Ideas / Closed-retired).
  lane: ".lane",
  // The lane's clickable header; its text is "<glyph> <label>" plus the count element.
  laneHeader: ".lane-header",
  // The count element inside a lane or group header.
  headerCount: ".cnt",
  // A lane header that is currently collapsed (the runner clicks these to expand).
  collapsedLaneHeader: { css: ".lane-header", textPrefix: "▸" },

  // ---- list --------------------------------------------------------------
  // One list group (In progress / Open / Closed / Retired / …, or stint / type groups).
  group: ".grp",
  // The group's clickable header.
  groupHeader: ".grp > header",
  // A group header that is currently collapsed.
  collapsedGroupHeader: { css: ".grp > header", textPrefix: "▸" },

  // ---- detail (record modal) ---------------------------------------------
  // The open record detail / dialog wrapper. Absent when nothing is open.
  detail: "[data-component='Modal'] [data-modal-dialog]",
  // Record id shown in the detail head (.mhead is our own markup inside the Modal content region).
  detailId: "[data-component='Modal'] [data-modal-content] .mhead .idm",
  // Record title shown in the detail.
  detailTitle: "[data-component='Modal'] [data-modal-title]",
  // The detail's close control.
  detailClose: "[data-component='Modal'] [data-modal-close]",
  // The "no records match" empty state.
  emptyState: ".empty",
};

// Which focused elements count as "a record" or "a lane header" in the keyboard walk.
export const focusRoles = {
  record: ".bcard, .actrow",
  laneHeader: ".lane-header, .grp > header",
};
