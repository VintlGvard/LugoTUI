local ui = require('ui')

return {
  api_version = 1,
  view = ui.column {
    ui.input { id = 'input', bind = 'query', placeholder = 'Text, or fail', height = 1 },
    ui.button { id = 'run', text = 'Run', on_click = 'run', height = 1 },
    ui.button { id = 'cancel', text = 'Cancel', on_click = 'cancel', height = 1 },
    ui.text { id = 'status', text = 'Idle', height = 1 },
    ui.text { id = 'result', text = '', height = 1 }
  },
  actions = {
    run = function()
      ui.patch('status', { text = 'Running' })
      host.dispatch('run', { text = ui.get('query') or '' })
    end,
    cancel = function() host.dispatch('cancel', {}) end,
    cancelled = function() ui.patch('status', { text = 'Cancelled' }) end,
    finished = function(event)
      ui.patch('status', { text = event.error and 'Error' or 'Done' })
      ui.patch('result', { text = event.error or event.text or '' })
    end
  }
}
