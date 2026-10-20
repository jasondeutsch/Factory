(() => {
  const projectSelect = document.getElementById('project');
  const tokenInput = document.getElementById('token');
  const message = document.getElementById('message');
  const taskBody = document.getElementById('tasks');
  const count = document.getElementById('count');
  let refreshTimer;
  let before = 0;
  let lastSequence = 0;
  let browsingOlderTasks = false;

  function cell(row, value, className) {
    const element = document.createElement('td');
    if (className) element.className = className;
    element.textContent = value || '—';
    row.appendChild(element);
    return element;
  }

  async function request(path) {
    const headers = {};
    if (tokenInput.value) headers.Authorization = `Bearer ${tokenInput.value}`;
    const response = await fetch(path, {headers, cache: 'no-store'});
    if (response.status === 401) throw new Error('Authentication required. Enter the Comms token above.');
    if (!response.ok) throw new Error(`Request failed (${response.status}).`);
    return response.json();
  }

  async function loadProjects() {
    try {
      const data = await request('/monitor/projects.json');
      message.className = 'statusline';
      const current = projectSelect.value;
      projectSelect.replaceChildren(new Option('Choose a project', ''));
      for (const name of data.projects || []) projectSelect.add(new Option(name, name));
      if ((data.projects || []).includes(current)) projectSelect.value = current;
      if (projectSelect.value) await loadTasks();
      else message.textContent = data.projects.length ? 'Choose a project to view its tasks.' : 'No tasks have been recorded yet.';
    } catch (error) {
      message.textContent = error.message;
      message.className = 'statusline error';
    }
  }

  async function loadTasks(older = false) {
    const project = projectSelect.value;
    if (!project) return;
    try {
      if (!older) {
        before = 0;
        browsingOlderTasks = false;
      }
      const previousCount = older ? taskBody.rows.length : 0;
      const cursor = older ? `&before=${before}` : '';
      const data = await request(`/monitor/tasks.json?project=${encodeURIComponent(project)}&limit=50${cursor}`);
      if (!older) taskBody.replaceChildren();
      if (!data.tasks.length && !older) {
        const row = taskBody.insertRow();
        const empty = row.insertCell();
        empty.colSpan = 8;
        empty.className = 'empty';
        empty.textContent = 'No tasks for this project.';
      }
      if (data.tasks.length) {
        lastSequence = data.tasks[data.tasks.length - 1].task.sequence;
        document.getElementById('older').hidden = data.tasks.length < 50;
      } else {
        document.getElementById('older').hidden = true;
      }
      for (const item of data.tasks) {
        const task = item.task;
        const row = taskBody.insertRow();
        const title = row.insertCell();
        const link = document.createElement('a');
        link.href = `/tasks/${encodeURIComponent(task.id)}/status`;
        link.textContent = task.name;
        title.appendChild(link);
        const summary = document.createElement('div');
        summary.className = 'muted task-summary';
        summary.textContent = task.summary;
        title.appendChild(summary);
        const state = cell(row, task.status);
        const badge = document.createElement('span');
        badge.className = `pill ${task.status}`;
        badge.textContent = task.status;
        state.replaceChildren(badge);
        cell(row, item.factory);
        cell(row, item.active_assignment_title ? `${item.active_stage}: ${item.active_assignment_title} · ${item.assignment_state} · ${item.assignee}` : 'No active assignment');
        cell(row, item.open_attention ? String(item.open_attention) : '0', item.open_attention ? 'attention' : '');
        cell(row, String(item.unread_messages));
        cell(row, item.latest_event ? `${item.latest_event}\n${item.latest_event_at}` : 'No recorded events');
        cell(row, task.updated_at);
      }
      if (older && data.tasks.length) browsingOlderTasks = true;
      before = lastSequence;
      const shown = previousCount + data.tasks.length;
      count.textContent = `${shown} task${shown === 1 ? '' : 's'} shown`;
      message.className = 'statusline';
      message.textContent = browsingOlderTasks
        ? `Updated ${new Date().toLocaleTimeString()}. Auto-refresh pauses while browsing older tasks; use Refresh to return to the latest.`
        : `Updated ${new Date().toLocaleTimeString()}. Refreshes every 10 seconds.`;
    } catch (error) {
      message.className = 'statusline error';
      message.textContent = error.message;
    }
  }

  function scheduleRefresh() {
    clearInterval(refreshTimer);
    refreshTimer = setInterval(() => {
      if (!browsingOlderTasks) loadTasks();
    }, 10000);
  }

  document.getElementById('refresh').addEventListener('click', () => { browsingOlderTasks = false; loadProjects(); scheduleRefresh(); });
  document.getElementById('older').addEventListener('click', () => loadTasks(true));
  projectSelect.addEventListener('change', () => { browsingOlderTasks = false; loadTasks(); });
  tokenInput.addEventListener('change', () => { browsingOlderTasks = false; loadProjects(); });
  loadProjects();
  scheduleRefresh();
})();
