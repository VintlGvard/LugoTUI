local ui = require('ui')
local pages = {'Overview', 'Text', 'Input', 'Layout', 'Runtime', 'Transport', 'Help', 'Collections', 'Motion', 'Time', 'Editors'}
local file_items = {
 {text = 'src/', path = 'src', is_dir = true, size = 4096, permissions = 'drwxr-xr-x'},
 {text = 'README.md', path = 'README.md', is_dir = false, size = 1842, permissions = '-rw-r--r--'},
 {text = 'palette.lua', path = 'palette.lua', is_dir = false, size = 528, permissions = '-rw-r--r--'},
}
local function event_text(e)
 local value = e.value or e.text or e.path or e.id or e.index or 'selected'
 if type(value) == 'table' then value = value.text or value.path or value.id or value[1] or 'row' end
 return tostring(value)
end
local function status(value) ui.patch('status', {text = value}) end
local function reset_clock(id)
 local key = (ui.get(id .. '_reset') or 0) + 1
 ui.set(id .. '_reset', key)
 ui.set(id .. '_running', false)
 ui.patch(id, {running = false, reset_key = key})
 local name = (id == 'catalog_timer') and 'Timer' or 'Stopwatch'
 status(name .. ' · reset (paused)')
 if id == 'catalog_timer' then
  ui.set('catalog_timer_result', 'Reset · ready for countdown')
  ui.patch('catalog_timer_result', {text = ui.get('catalog_timer_result')})
 end
end
local function toggle_clock(id)
 local running = not ui.get(id .. '_running')
 ui.set(id .. '_running', running)
 ui.patch(id, {running = running})
 local name = (id == 'catalog_timer') and 'Timer' or 'Stopwatch'
 status(name .. ' · ' .. (running and 'running' or 'paused'))
 if id == 'catalog_timer' then
  ui.set('catalog_timer_result', running and 'Counting down · 5s' or 'Paused')
  ui.patch('catalog_timer_result', {text = ui.get('catalog_timer_result')})
 end
end
local function progress_step(delta)
 local percent = math.max(0, math.min(1, (ui.get('catalog_percent') or 0.35) + delta))
 ui.set('catalog_percent', percent)
 ui.patch('catalog_progress', {percent = percent})
 ui.patch('status', {text = 'Motion · target ' .. tostring(math.floor(percent * 100 + 0.5)) .. '%'})
end
local commands = {'Focus prompt', 'Focus scratchpad', 'Clear draft', 'Show overview'}
for _, name in ipairs(pages) do commands[#commands + 1] = 'Page: ' .. name end
local navy, surface, card_bg, dark_surface = '#0e1726', '#162235', '#1d2d47', '#0c1320'
local cyan, amber, emerald, coral, purple, muted, white = '#38bdf8', '#fbbf24', '#34d399', '#f87171', '#a78bfa', '#94a3b8', '#f8fafc'
local function text(value, height)
 return ui.text {text = value, height = height, shrink = 0, wrap = 'word'}
end
local function heading(value)
 return ui.text {text = value, height = 1, shrink = 0, fg = cyan, bold = true}
end
local function button(id, label, action)
 return ui.button {id = id, text = label, on_click = action, height = 1, shrink = 0, bg = surface, fg = cyan}
end
local function card(id, title, description, target)
 return ui.panel {title = ' ' .. title .. ' ', grow = 1, basis = 20, min_width = 6, bg = surface,
  button(id, 'Explore ' .. target, 'open_' .. string.lower(target)), text(description, 1)}
end
local function runtime_modal()
 return ui.overlay {id = 'runtime_modal', hidden = true, modal = true, position = 'absolute', left = '10%', top = '15%', width = '80%', height = 8, z_index = 20, border = true, padding = 1, title = ' Runtime · new subtree ', bg = surface,
  text('Name this experiment. Enter accepts; Esc cancels.', 1),
  ui.input {id = 'runtime_name', bind = 'runtime_name', text = '', height = 1, shrink = 0, on_submit = 'runtime_accept', bg = navy},
  button('runtime_accept_button', 'Accept experiment', 'runtime_accept_current')}
end
local function layout_status_str()
 local dir = ui.get('layout_direction') or 'row'
 local j = ui.get('layout_justify') or 'space-between'
 local a = ui.get('layout_align') or 'center'
 local g = tostring(ui.get('layout_gap') or 1)
 return 'direction: ' .. dir .. '  |  justify: ' .. j .. '  |  align: ' .. a .. '  |  gap: ' .. g
end
local function page(name)
 local body = {}
 if name == 'Overview' then
  body = {
   ui.panel {title = ' LugoTUI Component Catalog ', height = 4, shrink = 0, bg = surface,
    ui.text {text = 'Retained terminal UI engine with embedded Lua scripting and native Go widgets.', fg = white, height = 1},
    ui.row {height = 1, shrink = 0, gap = 1,
     ui.text {text = ' Native Widgets ', bg = emerald, fg = navy, bold = true, width = 16},
     ui.text {text = ' Zero CGo / Pure Go ', bg = cyan, fg = navy, bold = true, width = 21},
     ui.text {text = ' Event Driven ', bg = purple, fg = navy, bold = true, width = 16},
     ui.text {text = ' Grapheme-Aware ', bg = amber, fg = navy, bold = true, width = 17}}},
   ui.row {height = 4, shrink = 0, gap = 1,
    card('overview_text', '01 / CONTENT & VIEWPORT', 'Spans, multi-script Unicode, soft-wrap & dual viewports.', 'Text'),
    card('overview_input', '02 / EDITORS & MASKS', 'Prompt, masked secrets, regex validation & autocomplete.', 'Input')},
   ui.row {height = 4, shrink = 0, gap = 1,
    card('overview_layout', '03 / FLEX ENGINE', 'Live justify, align, sizing constraints & layered z-index.', 'Layout'),
    card('overview_runtime', '04 / ATOMIC ACTIONS', 'ui.get/set models, patch, replace & transactional rollback.', 'Runtime')},
   ui.row {height = 1, shrink = 0, gap = 1,
    button('overview_transport', '05 / Host transport & bus', 'open_transport'),
    button('overview_help', '06 / Keyboard & safety', 'open_help')},
   text('Tab cycles focus · Ctrl+P opens command palette · F2 focuses persistent scratchpad · F5 reloads Lua.', 1),
  }
 elseif name == 'Text' then
  body = {
   heading('TYPOGRAPHY / COLOR, STYLE & MULTI-SCRIPT UNICODE'),
   ui.panel {title = ' Rich Styled Spans ', height = 3, shrink = 0, bg = surface,
    ui.text {id = 'rich_text', height = 1, shrink = 0, wrap = 'word', spans = {
     {text = ' LUGOTUI ', fg = navy, bg = cyan, bold = true},
     {text = ' bold ', bold = true}, {text = ' cyan ', fg = cyan},
     {text = ' emerald ', fg = emerald}, {text = ' amber ', fg = amber}}}},
   ui.panel {title = ' Multi-Script Language Sampler ', height = 7, shrink = 0, bg = surface,
    text('English   Design in terminal cells with zero flicker\nРусский   Привет, мир! Полноценный TUI на Go и Lua\n日本語    こんにちは世界 · ターミナルUIエンジン\n中文      你好，世界 · 终端界面与原生组件\nAccents   café · naïve · résumé · é (combined)', 5)},
   text('Wide glyphs occupy two terminal cells; combining accents stay with their grapheme clusters.', 1),
   heading('VIEWPORT / LONG LINES + SOFT WRAP + LINE NUMBERS'),
   ui.row {height = 8, shrink = 0, gap = 1,
    ui.panel {title = ' Log Viewport (Line numbers) ', grow = 1, bg = surface,
     ui.viewport {id = 'catalog_viewport', height = 6, shrink = 0, soft_wrap = false, show_line_numbers = true, bg = surface, fg = cyan,
      text = '00:01  renderer   ready   terminal cells: 100x30 viewport active ........................................ end of record\n00:02  layout     ready   flex geometry checked at wide and narrow terminal sizes\n00:03  transport  idle    host events stay explicit; no automatic response\n00:04  editor     ready   persistent scratchpad survives page navigation\n00:05  catalog    ready   native viewport can scroll both axes\n00:06  widgets    ready   native components active'}},
    ui.panel {title = ' Soft-Wrapped Document Viewport ', grow = 1, bg = surface,
     ui.viewport {id = 'catalog_wrapped_viewport', height = 6, shrink = 0, soft_wrap = true, bg = surface,
      text = 'Soft wrapping keeps long observations neatly inside the viewport: terminal glyphs 界界, combining accents é and emoji 🙂 remain cell-aware. Focus to scroll through records using arrows or vim keys (j/k, d/u, g/G).'}}},
   text('Focus a viewport: arrows or vim (j/k, d/u, g/G) scroll; PgUp / PgDn / Home / End or mouse wheel move through records.', 2),
  }
 elseif name == 'Input' then
  body = {
   heading('EDIT → BIND → OBSERVE'),
   text('Edit the field; preview shares demo_value. Enter reports on_submit.', 1),
   ui.panel {title = ' Reactive Model Binding ', height = 6, shrink = 0, bg = surface,
    ui.input {id = 'demo_input', bind = 'demo_value', text = 'Hello world', on_change = 'demo_change', on_submit = 'demo_submit', height = 1, shrink = 0, bg = dark_surface},
    ui.text {id = 'bound_preview', bind = 'demo_value', text = 'Hello world', height = 1, shrink = 0, wrap = 'word', fg = cyan},
    ui.text {id = 'callback_result', text = 'Callbacks report here.', height = 1, shrink = 0, wrap = 'word', fg = amber}},
   heading('SELECT / CHANGE SOURCE / CONTROLS'),
   ui.panel {title = ' List Selection & Dynamic State ', height = 8, shrink = 0, bg = surface,
    ui.list {id = 'demo_list', source = 'demo_items', items = {'Alpha', 'Beta', 'Gamma'}, on_select = 'demo_select', height = 3, shrink = 0},
    ui.row {height = 1, shrink = 0, gap = 1,
     button('source_button', 'Change list source', 'demo_source'),
     button('toggle_button', 'Toggle hidden / disabled', 'demo_toggle'),
     ui.button {id = 'disabled_button', text = ui.get('demo_enabled') and 'Enabled' or 'Disabled', disabled = not ui.get('demo_enabled'), on_click = 'demo_click', height = 1, shrink = 0, bg = dark_surface, fg = amber}},
    ui.text {id = 'hidden_sample', text = 'Hidden content revealed by ui.patch.', hidden = not ui.get('demo_enabled'), height = 1, shrink = 0, fg = emerald}},
   text('Editor tools: Shift selects; Ctrl+A selects all; Ctrl+Z / Ctrl+Y undo / redo. Paste or mouse-drag to select. Backspace removes one grapheme.', 2),
  }
 elseif name == 'Collections' then
  body = {
   heading('COLLECTIONS / TABLE, TREE, LIST & FILEPICKER'),
   text('Native rows, branches and filtered records. Enter selects; callbacks update status.', 1),
   ui.panel {title = ' Table / Structured Column Data ', height = 6, shrink = 0, bg = surface,
    ui.table {id = 'catalog_table', columns = {{title = 'Module', width = 18}, {title = 'State', width = 12}, {title = 'Owner', width = 16}, {title = 'Health', width = 10}},
     rows = {{'Renderer', '● Ready', 'Terminal (tcell)', '100%'}, {'Layout', '● Ready', 'Flex Engine', '100%'}, {'Transport', '○ Waiting', 'Host Bus', '99.8%'}, {'Native Widgets', '● Active', 'LugoTUI', '100%'}},
     on_select = 'collection_select', height = 4, shrink = 0, bg = surface, fg = cyan}},
   ui.row {height = 8, shrink = 0, gap = 1,
    ui.panel {title = ' Tree / Hierarchy ', grow = 1, bg = surface,
     ui.tree {id = 'catalog_tree', height = 6, shrink = 0, on_select = 'collection_select', items = {
      {id = 'workbench', text = '📁 lugotui/', expanded = true, children = {
       {id = 'views', text = '📁 internal/', expanded = true, children = {{id = 'engine', text = '📁 engine/'}, {id = 'runner', text = '📁 runner/'}}},
       {id = 'cmd', text = '📁 cmd/lugotui/'},
       {id = 'assets', text = '📁 internal/assets/'},
       {id = 'readme', text = '📄 README.md'}}}}}},
    ui.panel {title = ' Filterable List / Query ', grow = 1, bg = surface,
     ui.list {id = 'catalog_list', title = 'Signal queue', show_status = true, show_help = true, query = '',
      filterable = true, page_size = 3, height = 6, shrink = 0, on_select = 'collection_select', items = {
       {text = 'Cyan / active', description = 'Live terminal paint'}, {text = 'Amber / attention', description = 'Pending host response'},
       {text = 'Navy / canvas', description = 'Quiet background'}, {text = 'Emerald / ready', description = 'Normal operational state'}}}}},
   ui.panel {title = ' Filepicker / File Explorer (No FS sandbox breach) ', height = 6, shrink = 0, bg = surface,
    ui.filepicker {id = 'catalog_files', items = file_items, height = 4, shrink = 0, show_size = true, show_permissions = true, on_select = 'collection_select', on_navigate = 'file_navigate'}},
   text('Paths are supplied Lua records with size and permissions. Open src/ to swap in another supplied listing.', 2),
  }
 elseif name == 'Motion' then
  body = {
   heading('MOTION / 12 SPINNER PRESETS & CURSOR'),
   text('Go ticks native animation. No Lua worker, sleep or synthetic host event.', 1),
   ui.panel {title = ' Terminal Cursor & Spinner Gallery ', height = 5, shrink = 0, bg = surface,
    ui.row {height = 1, shrink = 0, gap = 2,
     ui.cursor {id = 'catalog_cursor', mode = 'blink', char = '█', blink_speed = 530, fg = amber},
     ui.text {text = 'Native cursor · 530 ms cadence', width = 32, fg = white},
     ui.spinner {id = 'catalog_spinner', preset = 'mini_dot', interval = 90, width = 2, height = 1, fg = cyan},
     ui.text {text = 'mini_dot', width = 10, fg = muted},
     ui.spinner {id = 'catalog_sp_line', preset = 'line', interval = 100, width = 2, height = 1, fg = amber},
     ui.text {text = 'line', width = 8, fg = muted},
     ui.spinner {id = 'catalog_sp_dot', preset = 'dot', interval = 80, width = 2, height = 1, fg = emerald},
     ui.text {text = 'dot', width = 8, fg = muted}},
    ui.row {height = 1, shrink = 0, gap = 2,
     ui.spinner {id = 'catalog_sp_globe', preset = 'globe', interval = 120, width = 2, height = 1, fg = coral},
     ui.text {text = 'globe', width = 10, fg = muted},
     ui.spinner {id = 'catalog_sp_moon', preset = 'moon', interval = 150, width = 2, height = 1, fg = purple},
     ui.text {text = 'moon', width = 10, fg = muted},
     ui.spinner {id = 'catalog_sp_monkey', preset = 'monkey', interval = 180, width = 2, height = 1, fg = amber},
     ui.text {text = 'monkey', width = 10, fg = muted},
     ui.spinner {id = 'catalog_sp_pulse', preset = 'pulse', interval = 100, width = 2, height = 1, fg = cyan},
     ui.text {text = 'pulse', width = 10, fg = muted},
     ui.spinner {id = 'catalog_sp_meter', preset = 'meter', interval = 100, width = 2, height = 1, fg = emerald},
     ui.text {text = 'meter', width = 8, fg = muted}}},
   heading('PROGRESS / INTERPOLATED COLOR GRADIENT'),
   ui.panel {title = ' Dynamic Progress Bar ', height = 4, shrink = 0, bg = surface,
    ui.progress {id = 'catalog_progress', percent = ui.get('catalog_percent') or 0.35, animated = true,
     gradient = {cyan, amber}, percent_format = ' %3.0f%%', height = 1, shrink = 0},
    ui.row {height = 1, shrink = 0, gap = 1,
     button('progress_less', '− 10%', 'progress_less'), button('progress_more', '+ 10%', 'progress_more'),
     button('progress_reset', 'Reset to zero', 'progress_reset')}},
   heading('PAGINATORS / DUAL MODES'),
   ui.panel {title = ' Record Pagination Indicators ', height = 5, shrink = 0, bg = surface,
    ui.row {height = 1, shrink = 0, gap = 2,
     ui.text {text = 'Dots mode:   ', fg = muted, width = 13},
     ui.paginator {id = 'catalog_dots', mode = 'dots', page = 0, per_page = 5, total = 23, on_change = 'page_change', height = 1, shrink = 0, fg = cyan}},
    ui.row {height = 1, shrink = 0, gap = 2,
     ui.text {text = 'Numeric mode:', fg = muted, width = 13},
     ui.paginator {id = 'catalog_numeric', mode = 'numeric', page = 0, per_page = 5, total = 23, on_change = 'page_change', height = 1, shrink = 0, fg = amber}},
    ui.text {id = 'catalog_page_result', text = 'Page 1 / 5 · 23 records · 5 per page', height = 1, shrink = 0, fg = emerald}},
   text('Focus paginator: arrows or vim (h/j/k/l) / PgUp / PgDn / Home / End. Callback syncs both; API pages start at zero.', 2),
  }
 elseif name == 'Time' then
  body = {
   heading('TIME / PRECISION CLOCKS (SUBSECOND & DURATION FORMATS)'),
   text('Native countdown timer and count-up stopwatch with subsecond precision. Space or Enter toggles focused clock; r resets it.', 2),
   ui.panel {title = ' Countdown Timer / 5.0 Seconds ', height = 6, shrink = 0, bg = surface,
    ui.row {height = 1, shrink = 0, gap = 2,
     ui.text {text = 'Remaining:', fg = muted, width = 12},
     ui.timer {id = 'catalog_timer', duration = 5000, running = ui.get('catalog_timer_running') or false,
      reset_key = ui.get('catalog_timer_reset') or 0, on_timeout = 'timer_timeout', on_change = 'clock_change', subsecond = true, height = 1, shrink = 0, fg = amber}},
    ui.row {height = 1, shrink = 0, gap = 1,
     button('timer_toggle', 'Start / pause', 'timer_toggle'), button('timer_reset', 'Reset countdown', 'timer_reset')},
    ui.text {id = 'catalog_timer_result', text = ui.get('catalog_timer_result') or 'Ready · timeout callback reports here', height = 1, shrink = 0, fg = cyan}},
   ui.panel {title = ' Stopwatch / Elapsed Time ', height = 5, shrink = 0, bg = surface,
    ui.row {height = 1, shrink = 0, gap = 2,
     ui.text {text = 'Elapsed:', fg = muted, width = 12},
     ui.stopwatch {id = 'catalog_stopwatch', running = ui.get('catalog_stopwatch_running') or false,
      reset_key = ui.get('catalog_stopwatch_reset') or 0, on_change = 'clock_change', subsecond = true, height = 1, shrink = 0, fg = cyan}},
    ui.row {height = 1, shrink = 0, gap = 1,
     button('stopwatch_toggle', 'Start / pause', 'stopwatch_toggle'), button('stopwatch_reset', 'Reset elapsed', 'stopwatch_reset')}},
   text('Clocks belong to this page. Leaving it removes tick demand; model keeps control choices, not hidden background work.', 3),
  }
 elseif name == 'Editors' then
  body = {
   heading('EDITORS / GUARDED INPUT & VALIDATION'),
   text('Prompt, placeholder, limits, suggestions, validation and cursor modes are native input features.', 2),
   ui.panel {title = ' Input Controls ', height = 9, shrink = 0, bg = surface,
    ui.input {id = 'catalog_suggest', bind = 'catalog_command', prompt = 'Run › ', placeholder = 'build, bench or browse',
     suggestions = {'build', 'bench', 'browse'}, char_limit = 24, cursor_mode = 'blink', on_submit = 'editor_submit', height = 1, shrink = 0, bg = dark_surface},
    ui.input {id = 'catalog_password', prompt = 'Key › ', placeholder = 'Masked local sample', echo = 'password',
     char_limit = 32, height = 1, shrink = 0, bg = dark_surface},
    ui.input {id = 'catalog_validated', prompt = 'Tag › ', placeholder = 'lowercase letters, digits, hyphens',
     validation_pattern = '^[a-z0-9-]+$', validation_error = 'Use lowercase letters, digits or hyphens',
     char_limit = 20, on_change = 'editor_validate', on_submit = 'editor_submit', height = 1, shrink = 0, bg = dark_surface},
    ui.text {id = 'catalog_editor_result', text = 'Validation and submit callbacks report here.', height = 1, shrink = 0, fg = amber}},
   ui.panel {title = ' Textarea / Code Editor (Line numbers + cursor) ', height = 5, shrink = 0, bg = surface,
    ui.textarea {id = 'catalog_editor_textarea', text = 'Multi-line code editor\nLine numbers and cursor\nNative multiline editing', height = 3, shrink = 0, line_numbers = true, cursor_mode = 'blink', bg = dark_surface}},
   ui.panel {title = ' Key Help Reference ', height = 7, shrink = 0, bg = surface,
    ui.help {id = 'catalog_help', full = true, height = 5, shrink = 0, keys = {
     {keys = {'F1'}, help = 'Open keyboard and safety reference', action = 'help', enabled = true},
     {keys = {'F2'}, help = 'Focus persistent scratchpad', action = 'editor', enabled = true},
     {keys = {'Ctrl+P'}, help = 'Find a workbench command', action = 'palette', enabled = true},
     {keys = {'x'}, help = 'Reserved / disabled example', action = 'help', enabled = false}}},
    ui.text {text = 'Focus help and press ? to toggle compact / full mode.', height = 1, shrink = 0, fg = muted}},
  }
 elseif name == 'Layout' then
  if ui.get('layout_status') == nil then ui.set('layout_status', layout_status_str()) end
  body = {
   heading('FLEX LAB / LIVE CONTROLS'),
   text('Live-patch container direction, alignment, distribution and gap. Choices survive page changes.', 1),
   ui.row {height = 1, shrink = 0, gap = 1,
    ui.text {text = 'Justify: ', width = 9, fg = muted, bold = true},
    button('justify_start', 'Start', 'justify_start'),
    button('justify_center', 'Center', 'justify_center'),
    button('justify_end', 'End', 'justify_end'),
    button('justify_between', 'Space between', 'justify_between'),
    button('justify_around', 'Space around', 'justify_around')},
   ui.row {height = 1, shrink = 0, gap = 1,
    ui.text {text = 'Align:   ', width = 9, fg = muted, bold = true},
    button('align_start', 'Top (start)', 'align_start'),
    button('align_center', 'Middle (center)', 'align_center'),
    button('align_end', 'Bottom (end)', 'align_end'),
    button('align_stretch', 'Stretch', 'align_stretch')},
   ui.row {height = 1, shrink = 0, gap = 1,
    ui.text {text = 'Dir/Gap: ', width = 9, fg = muted, bold = true},
    button('direction_row', 'Row', 'direction_row'),
    button('direction_column', 'Column', 'direction_column'),
    button('gap_0', 'Gap: 0', 'gap_0'),
    button('gap_1', 'Gap: 1', 'gap_1'),
    button('gap_2', 'Gap: 2', 'gap_2'),
    button('layout_reset', 'Reset', 'layout_reset')},
   ui.row {height = 1, shrink = 0, gap = 1,
    ui.text {text = ' Live State: ', bg = purple, fg = navy, bold = true, width = 13},
    ui.text {id = 'layout_status_badge', bind = 'layout_status', height = 1, fg = emerald, bold = true}},
   ui.panel {title = ' Live Flexbox Specimen Container ', height = 8, shrink = 0, bg = surface,
    ui.box {id = 'layout_specimen', direction = ui.get('layout_direction') or 'row', height = 6, shrink = 0, padding = 1,
     gap = ui.get('layout_gap') or 1, justify = ui.get('layout_justify') or 'space-between',
     align = ui.get('layout_align') or 'center', bg = dark_surface,
     ui.text {text = ' [1] Fixed 16x2 ', width = 16, height = 2, shrink = 0, fg = navy, bg = cyan, bold = true},
     ui.text {text = ' [2] Auto Stretch ', shrink = 0, fg = navy, bg = amber, bold = true},
     ui.text {text = ' [3] Fixed 14x1 ', width = 14, height = 1, shrink = 0, fg = navy, bg = emerald, bold = true}}},
   heading('SIZING & FLEX DISTRIBUTION'),
   text('Percentage widths compute from parent; grow distributes remaining space according to weight ratios.', 1),
   ui.row {height = 5, shrink = 0, gap = 1, align = 'stretch',
    ui.panel {title = ' width = 25% ', width = '25%', min_width = 8, max_width = 32, bg = card_bg,
     ui.text {text = 'Proportional width', fg = cyan, height = 1},
     ui.text {text = 'bounds: 8..32', fg = muted, height = 1}},
    ui.panel {title = ' grow = 1 ', grow = 1, basis = 10, shrink = 1, min_width = 6, bg = card_bg,
     ui.text {text = '1x remaining share', fg = emerald, height = 1},
     ui.text {text = 'basis=10 shrink=1', fg = muted, height = 1}},
    ui.panel {title = ' grow = 2 ', grow = 2, basis = 10, shrink = 2, max_width = 50, bg = card_bg,
     ui.text {text = '2x remaining share', fg = amber, height = 1},
     ui.text {text = 'basis=10 shrink=2', fg = muted, height = 1}}},
   heading('CROSS-AXIS OVERRIDES & MARGINS'),
   text('align_self overrides container align per-item. Margin adds external spacing outside the box.', 1),
   ui.row {height = 5, shrink = 0, gap = 1, bg = dark_surface, padding = 1,
    ui.text {text = ' ↑ self: start ', width = 16, height = 1, align_self = 'start', fg = navy, bg = cyan},
    ui.text {text = ' ↕ self: center (m=1) ', width = 24, height = 1, align_self = 'center', margin = 1, fg = navy, bg = amber},
    ui.text {text = ' ↓ self: end ', width = 15, height = 1, align_self = 'end', fg = navy, bg = coral},
    ui.text {text = ' ↕ stretch ', width = 12, align_self = 'stretch', fg = navy, bg = purple}},
   heading('LAYERS & STACKING CONTEXT'),
   text('Absolute layers take no flow space; coordinates are relative to parent. Higher z_index paints on top.', 2),
   ui.box {height = 6, shrink = 0, position = 'relative', bg = dark_surface, padding = 1,
    ui.text {text = 'Base document flow (z_index = 0, in-flow content)\nElements under absolute overlays remain visible in background.', height = 2, fg = muted},
    ui.panel {position = 'absolute', left = 5, top = 1, width = 38, height = 3, z_index = 1, title = ' Layer 1 · z_index = 1 ', bg = surface,
     ui.text {text = 'left=5, top=1 · over base flow', fg = cyan}},
    ui.panel {position = 'absolute', left = 24, top = 2, width = 40, height = 3, z_index = 2, title = ' Layer 2 · z_index = 2 ', bg = card_bg,
     ui.text {text = 'left=24, top=2 · floats over Layer 1', fg = amber}}},
   text('Padding is inside; margin is outside; gap separates children. Absolute layers consume no flow space. Higher z_index paints last.', 2),
  }
 elseif name == 'Runtime' then
  body = {
   heading('SMALL ACTIONS / ATOMIC COMMITS'),
   text('ui.get / set holds model state. ui.patch edits props; ui.replace swaps a subtree. Persistent editors live outside this page.', 2),
   ui.panel {title = ' Reactive Counter & Subtree Swap ', height = 7, shrink = 0, bg = surface,
    ui.text {id = 'counter_label', text = 'Counter: ' .. tostring(ui.get('demo_count') or 0), height = 1, shrink = 0, fg = amber},
    button('counter_button', 'get / set / patch counter', 'demo_count'),
    ui.panel {id = 'replace_slot', title = ' Replace target ', height = 3, shrink = 0, text('Original subtree', 1)},
    ui.row {height = 1, shrink = 0, gap = 1,
     button('replace_button', 'Replace subtree', 'demo_replace'),
     button('focus_button', 'Focus persistent scratchpad', 'editor'),
     button('modal_button', 'Open modal command palette', 'palette')}},
   heading('REPLACE + FOCUS IN ONE ACTION'),
   ui.panel {title = ' Transactional Experiments ', height = 6, shrink = 0, bg = surface,
    button('runtime_modal_button', 'Create experiment…', 'runtime_open'),
    ui.text {id = 'runtime_result', text = ui.get('runtime_result') or 'No experiment accepted yet.', height = 1, shrink = 0, wrap = 'word', fg = amber},
    button('rollback_button', 'Try rollback (expected error)', 'demo_rollback')},
   text('Intentional failure after set, patch, focus and host.dispatch. Model, UI, focus and events stay unchanged. Runner shows a diagnostic; successful F5 reload clears it.', 2),
   text('F5: Go reloads external Lua transactionally. Matching IDs retain edits; bad reload keeps the live UI. Lua globals / upvalues are not rolled back; store state with ui.set.', 2),
  }
 elseif name == 'Transport' then
  body = {
   heading('HOST EVENTS / NO FAKE BACKGROUND WORK'),
   text('Send calls host.dispatch("message.send", {text=...}). A connected host drains events and replies via agent.text. Chunks append to the output below.', 2),
   ui.row {height = 1, shrink = 0, gap = 1,
    button('host_button', 'Dispatch demo.ping host event', 'demo_dispatch'),
    button('agent_button', 'Simulate incoming agent.text locally', 'demo_agent')},
   heading('GO OWNS WORKERS'),
   text('Runtime.StartTask runs context-aware Go work. Owner receives TaskResults and calls HandleTask; result reaches a named Lua action. Close cancels tasks. Lua cannot start workers.', 3),
   heading('LOCAL SOCKET / EXPLICIT ACCESS'),
   text('Without --sandbox, enable --socket for snapshot, revision-checked apply (patch / replace), message and events. Socket permissions: 0600. Only trusted local clients should connect.', 3),
   heading('SANDBOX / PROCESS BOUNDARY'),
   text('--sandbox uses bubblewrap for the whole process. Network and host directories are unavailable; socket transport is disabled. Embedded Lua has no os / io / arbitrary require; only require("ui").', 3),
  }
 else
  body = {
   heading('NAVIGATE'),
   text('Tab / Shift+Tab: focus. Navigator: arrows + Enter or click. Ctrl+P: type to filter commands, Tab to list, Enter to run. Esc closes a modal and restores focus.', 3),
   heading('SHORTCUTS'),
   text('F1 help · F2 scratchpad · F5 reload external app · Ctrl+Q quit. Resize freely; focus page scroll and use arrows / PgUp / PgDn / Home or mouse wheel.', 3),
   heading('EDIT'),
   text('Arrows / Home / End move; Shift extends selection. Ctrl+A selects all. Ctrl+Z / Ctrl+Y undo / redo. Backspace / Delete remove graphemes. Paste inserts text; mouse drag selects. Enter submits input or adds a textarea newline.', 4),
   heading('SELECT'),
   text('Lists: arrows / Home / End / PgUp / PgDn, then Enter or click. Hidden and disabled controls skip focus. Scratchpad and prompt remain outside the replaceable page.', 3),
   heading('SAFETY'),
   text('Rollback intentionally reports an error without committing state or events. Go owns tasks, sockets and sandbox. Lua dispatches host events; it does not gain host privileges.', 3),
  }
 end
 return ui.panel {id = 'active_page', title = ' ' .. name .. ' / arrows · wheel ', grow = 1,
  ui.scroll {id = 'page_scroll', grow = 1, gap = 1, children = body}}
end
local function navigate(name)
 local found = false
 for _, value in ipairs(pages) do if value == name then found = true end end
 if not found then error('unknown demo page: ' .. tostring(name)) end
 ui.set('demo_page', name)
 ui.replace('active_page', page(name))
 status(name .. ' · Tab focus · Ctrl+P commands · F2 editor · F5 reload')
end
local function layout(prop, value)
 ui.set('layout_' .. prop, value)
 ui.patch('layout_specimen', {[prop] = value})
 ui.set('layout_status', layout_status_str())
 status('Flex lab · ' .. prop .. ' = ' .. tostring(value))
end
local function agent(value)
 ui.set('response', (ui.get('response') or '') .. value)
 ui.patch('answer', {text = ui.get('response')})
 status('Agent response received · Ctrl+P commands')
end
local function send(value)
 if not value or value == '' then return end
 ui.set('response', '')
 ui.patch('answer', {text = 'You: ' .. value .. '\nWaiting for connected agent; see Transport.'})
 host.dispatch('message.send', {text = value})
 ui.set('draft', '')
 status('Request queued · no automatic agent reply')
end
local function accept(value)
 if not value or not string.find(value, '%S') then status('Enter an experiment name before accepting'); return end
 ui.set('runtime_result', 'Experiment: ' .. value)
 ui.patch('runtime_result', {text = ui.get('runtime_result')})
 ui.patch('runtime_modal', {hidden = true})
 status('Experiment accepted · Esc cancels without saving')
end
local app = {
 api_version = 1,
 keymap = {['Ctrl+P'] = 'palette', ['F1'] = 'help', ['F2'] = 'editor'},
 view = ui.box {id = 'root', bg = navy,
  ui.column {grow = 1, gap = 0,
   ui.row {height = 1, shrink = 0, bg = '#090d16', gap = 1,
    ui.text {text = ' LUGO·TUI ', bg = cyan, fg = navy, bold = true, width = 10},
    ui.text {text = 'WIDGET CATALOG', fg = amber, bold = true, width = 14},
    ui.text {text = '│', fg = '#1e293b', width = 1},
    ui.text {text = '15 Components', fg = emerald, width = 14},
    ui.text {text = '│', fg = '#1e293b', width = 1},
    ui.text {text = 'Event Driven', fg = purple, width = 14},
    ui.text {text = '│', fg = '#1e293b', width = 1},
    ui.text {text = 'F1 Help · Ctrl+P Commands · F2 Scratchpad · F5 Reload', fg = muted, grow = 1},
    ui.text {text = 'v1.0.0', fg = muted, width = 7}},
   ui.row {grow = 1, gap = 1,
    ui.panel {title = ' Navigator ', width = '20%', min_width = 12, max_width = 24, bg = surface,
     ui.list {id = 'tasks', grow = 1, items = pages, on_select = 'select'}},
    ui.column {grow = 1,
     page(ui.get('demo_page') or 'Overview'),
     ui.panel {title = ' Agent / Host Output ', height = 4, shrink = 1, bg = surface,
      ui.text {id = 'answer', text = ui.get('response') or 'Send a prompt to a connected host. Transport explains how.', grow = 1, wrap = 'word', fg = cyan}},
     ui.panel {id = 'editor_panel', title = ' 📝 Scratchpad / F2 / Persistent ', height = 4, shrink = 1, bg = surface,
      ui.textarea {id = 'notes', bind = 'notes', text = 'Try: café · Привет · こんにちは · 你好\nSwitch pages; your notes stay here.', grow = 1, on_change = 'notes_changed', bg = dark_surface}},
    }},
   ui.row {height = 1, shrink = 0, gap = 1, bg = dark_surface,
    ui.text {text = ' ❯', width = 3, fg = amber, bold = true},
    ui.input {id = 'prompt', bind = 'draft', placeholder = 'Type a message or command (or press Ctrl+P)...', grow = 1, on_submit = 'send', bg = surface},
    ui.button {id = 'send_button', text = ' Send ', width = 8, on_click = 'send_current', bg = cyan, fg = navy}},
   ui.row {height = 1, shrink = 0, bg = '#090d16',
    ui.text {id = 'status', height = 1, shrink = 0, text = 'Tab focus · Ctrl+P commands · F1 help · F2 scratchpad · F5 reload · Ctrl+Q quit', fg = muted, grow = 1}}},
  ui.overlay {id = 'palette', hidden = true, modal = true, position = 'absolute', left = '10%', top = '10%', width = '80%', height = '70%', z_index = 10, border = true, padding = 1, title = ' Commands / Esc close ', bg = surface,
   ui.input {id = 'command_filter', height = 1, shrink = 0, bind = 'command_filter', on_change = 'filter_commands', bg = dark_surface},
   ui.list {id = 'commands', grow = 1, items = commands, on_select = 'command'}},
  runtime_modal()},
 actions = {
  collection_select = function(e) status('Collection · ' .. event_text(e)) end,
  file_navigate = function(e)
   local path = e.path or ''
   local items = file_items
   if path == 'src' then items = {{text = '../', path = '', is_dir = true, permissions = 'drwxr-xr-x'}, {text = 'app.lua', path = 'src/app.lua', is_dir = false, size = 12400, permissions = '-rw-r--r--'}} end
   ui.patch('catalog_files', {items = items})
   status('Supplied listing · ' .. (path == '' and '/' or path) .. ' · no filesystem access')
  end,
  progress_less = function() progress_step(-0.1) end,
  progress_more = function() progress_step(0.1) end,
  progress_reset = function() ui.set('catalog_percent', 0); ui.patch('catalog_progress', {percent = 0}); status('Motion · target 0%') end,
  page_change = function(e)
   local current = e.page or 0
   ui.patch('catalog_dots', {page = current})
   ui.patch('catalog_numeric', {page = current})
   ui.patch('catalog_page_result', {text = 'Page ' .. tostring(current + 1) .. ' / 5 · 23 records · 5 per page'})
   status('Paginator · zero-based page ' .. tostring(current))
  end,
  clock_change = function(e)
   if e.id == 'catalog_timer' then
    ui.set('catalog_timer_running', e.running)
    status('Timer · ' .. (e.running and 'running' or 'paused'))
   elseif e.id == 'catalog_stopwatch' then
    ui.set('catalog_stopwatch_running', e.running)
    status('Stopwatch · ' .. (e.running and 'running' or 'paused'))
   end
  end,
  timer_toggle = function() toggle_clock('catalog_timer') end,
  stopwatch_toggle = function() toggle_clock('catalog_stopwatch') end,
  timer_reset = function()
   reset_clock('catalog_timer')
   ui.set('catalog_timer_result', 'Reset · ready for another run')
   ui.patch('catalog_timer_result', {text = ui.get('catalog_timer_result')})
  end,
  stopwatch_reset = function() reset_clock('catalog_stopwatch') end,
  timer_timeout = function()
   ui.set('catalog_timer_running', false)
   ui.set('catalog_timer_result', 'Countdown complete · on_timeout fired')
   ui.patch('catalog_timer', {running = false})
   ui.patch('catalog_timer_result', {text = ui.get('catalog_timer_result')})
   status('Time · countdown complete')
  end,
  editor_submit = function(e) ui.patch('catalog_editor_result', {text = 'Submitted: ' .. event_text(e)}) end,
  editor_validate = function(e) ui.patch('catalog_editor_result', {text = e.error and e.error ~= '' and e.error or 'Valid tag: ' .. event_text(e)}) end,
  select = function(e) navigate(e.value) end,
  justify_start = function() layout('justify', 'start') end,
  justify_center = function() layout('justify', 'center') end,
  justify_end = function() layout('justify', 'end') end,
  justify_between = function() layout('justify', 'space-between') end,
  justify_around = function() layout('justify', 'space-around') end,
  align_start = function() layout('align', 'start') end,
  align_center = function() layout('align', 'center') end,
  align_end = function() layout('align', 'end') end,
  align_stretch = function() layout('align', 'stretch') end,
  direction_row = function() layout('direction', 'row') end,
  direction_column = function() layout('direction', 'column') end,
  gap_0 = function() layout('gap', 0) end,
  gap_1 = function() layout('gap', 1) end,
  gap_2 = function() layout('gap', 2) end,
  layout_reset = function()
   ui.set('layout_direction', 'row')
   ui.set('layout_justify', 'space-between')
   ui.set('layout_align', 'center')
   ui.set('layout_gap', 1)
   ui.patch('layout_specimen', {direction = 'row', justify = 'space-between', align = 'center', gap = 1})
   ui.set('layout_status', layout_status_str())
   status('Flex lab · reset to defaults')
  end,
  runtime_open = function()
   ui.set('runtime_name', '')
   ui.replace('runtime_modal', runtime_modal())
   ui.patch('runtime_modal', {hidden = false})
   ui.focus('runtime_name')
  end,
  runtime_accept = function(e) accept(e.value) end,
  runtime_accept_current = function() accept(ui.get('runtime_name')) end,
  palette = function()
   ui.set('command_filter', '')
   ui.patch('commands', {items = commands})
   ui.patch('palette', {hidden = false})
  end,
  editor = function() ui.focus('notes') end,
  help = function() navigate('Help') end,
  command = function(e)
   ui.patch('palette', {hidden = true})
   if e.value == 'Focus prompt' then ui.focus('prompt')
   elseif e.value == 'Focus scratchpad' then ui.focus('notes')
   elseif e.value == 'Clear draft' then ui.set('draft', '')
   elseif e.value == 'Show overview' then navigate('Overview')
   elseif string.sub(e.value or '', 1, 6) == 'Page: ' then navigate(string.sub(e.value, 7)) end
  end,
  filter_commands = function(e)
   local found = {}
   for _, command in ipairs(commands) do
    if string.find(string.lower(command), string.lower(e.value or ''), 1, true) then found[#found + 1] = command end
   end
   ui.patch('commands', {items = found})
  end,
  send = function(e) send(e.value) end,
  send_current = function() send(ui.get('draft')) end,
  notes_changed = function() status('Scratchpad saved in bound model · F5 preserves edits') end,
  demo_change = function(e) ui.patch('callback_result', {text = 'on_change: ' .. e.value}) end,
  demo_submit = function(e) ui.patch('callback_result', {text = 'on_submit: ' .. e.value}) end,
  demo_select = function(e) ui.patch('callback_result', {text = 'on_select: ' .. tostring(e.index) .. ' / ' .. e.value}) end,
  demo_source = function() ui.set('demo_items', {'Model source updated', 'Delta', 'Select with Enter'}) end,
  demo_click = function() ui.patch('callback_result', {text = 'on_click: enabled button'}) end,
  demo_toggle = function()
   local enabled = not ui.get('demo_enabled')
   ui.set('demo_enabled', enabled)
   ui.patch('disabled_button', {disabled = not enabled, text = enabled and 'Enabled' or 'Disabled'})
   ui.patch('hidden_sample', {hidden = not enabled})
  end,
  demo_count = function()
   local count = (ui.get('demo_count') or 0) + 1
   ui.set('demo_count', count)
   ui.patch('counter_label', {text = 'Counter: ' .. tostring(count)})
  end,
  demo_replace = function()
   ui.replace('replace_slot', ui.panel {id = 'replace_slot', title = ' Replaced subtree ', height = 3, shrink = 0,
    ui.text {id = 'replacement_text', text = 'ui.replace committed; surrounding editors untouched.', height = 1}})
  end,
  demo_rollback = function()
   ui.set('demo_count', -999)
   ui.patch('counter_label', {text = 'This must never commit'})
   ui.focus('prompt')
   host.dispatch('demo.rollback', {must_not_escape = true})
   error('Demo rollback: intentional failure; model, UI, focus and event queue preserved')
  end,
  demo_dispatch = function() host.dispatch('demo.ping', {page = ui.get('demo_page')}); status('Dispatched demo.ping; host drains event queue') end,
  demo_agent = function() agent('Local agent.text sample: stream response active\n') end,
  ['agent.text'] = function(e) agent(e.text or '') end,
 }
}

for _, name in ipairs(pages) do
 local target = name
 app.actions['open_' .. string.lower(target)] = function() navigate(target) end
end
return app
