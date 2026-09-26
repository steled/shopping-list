(function () {
  'use strict';

  var U = window.UI;
  var h = U.h;
  var root = document.getElementById('recipes-root');
  var summary = document.getElementById('recipes-summary');
  var addForm = document.getElementById('add-recipe-form');
  var addInput = document.getElementById('new-recipe-name');

  var state = { recipes: [], categories: [], open: null, confirmDelete: null };
  var saveTimers = {};

  async function load() {
    try {
      var res = await Promise.all([U.api('GET', '/api/recipes'), U.api('GET', '/api/categories')]);
      state.recipes = res[0];
      state.categories = res[1];
      render();
    } catch (err) {
      console.error('load recipes failed', err);
      root.replaceChildren(h('p', { class: 'banner banner-warn' }, 'Die Rezepte konnten nicht geladen werden. Bitte die Seite neu laden.'));
    }
  }

  // Saves are debounced per recipe so typing doesn't send a request per key.
  function scheduleSave(r) {
    clearTimeout(saveTimers[r.id]);
    saveTimers[r.id] = setTimeout(function () { save(r); }, 400);
  }

  // Saves of one recipe run one after another, so an older request can never
  // overwrite a newer state.
  var saveChains = {};
  function save(r) {
    clearTimeout(saveTimers[r.id]);
    delete saveTimers[r.id];
    saveChains[r.id] = (saveChains[r.id] || Promise.resolve()).then(function () { return doSave(r); });
    return saveChains[r.id];
  }

  async function doSave(r) {
    try {
      await U.api('PUT', '/api/recipes/' + r.id, {
        name: r.name,
        active: r.active,
        ingredients: r.ingredients
          .filter(function (g) { return g.name.trim() !== ''; })
          .map(function (g) {
            return { name: g.name.trim(), quantity: g.quantity, amount: g.amount.trim(), category_id: g.category_id, pantry: g.pantry };
          }),
      });
    } catch (err) {
      console.error('save recipe failed', err);
      U.toast('„' + r.name + '“ konnte nicht gespeichert werden.');
    }
  }

  function renderSummary() {
    var active = state.recipes.filter(function (r) { return r.active; }).length;
    var text = active + (active === 1 ? ' aktives Gericht. ' : ' aktive Gerichte. ') +
      'Ein Monat hat 8 bis 10 Wochenend-Termine. Pausierte Gerichte werden beim Würfeln übersprungen.';
    summary.textContent = text;
  }

  function render() {
    renderSummary();
    if (state.recipes.length === 0) {
      root.replaceChildren(h('p', { class: 'empty-hint empty-hint-recipes' }, 'Noch keine Gerichte – legt oben euer erstes an.'));
      return;
    }
    root.replaceChildren.apply(root, state.recipes.map(renderRecipe));
  }

  function renderRecipe(r) {
    var open = state.open === r.id;
    var activeCb = h('input', { type: 'checkbox', checked: r.active });
    activeCb.addEventListener('change', function () {
      r.active = activeCb.checked;
      card.classList.toggle('is-inactive', !r.active);
      sub.textContent = subText(r);
      renderSummary();
      save(r);
    });
    var sub = h('span', { class: 'recipe-sub' }, subText(r));
    var card = h('section', { class: 'recipe' + (r.active ? '' : ' is-inactive'), 'data-recipe': r.id },
      h('div', { class: 'recipe-head' },
        h('button', { type: 'button', class: 'recipe-toggle', 'aria-expanded': String(open), onclick: function () {
          state.open = open ? null : r.id;
          state.confirmDelete = null;
          render();
        } }, h('span', { class: 'recipe-title' }, r.name), sub),
        h('label', { class: 'switch', title: 'Beim Würfeln berücksichtigen' }, activeCb, 'aktiv')),
      open && renderBody(r));
    return card;
  }

  function subText(r) {
    var n = r.ingredients.filter(function (g) { return g.name.trim() !== ''; }).length;
    var parts = [n === 1 ? '1 Zutat' : n + ' Zutaten'];
    if (n > 0 && r.ingredients.every(function (g) { return !g.amount; })) parts.push('ohne Mengen');
    if (!r.active) parts.push('pausiert');
    return parts.join(' · ');
  }

  function renderBody(r) {
    var nameInput = h('input', { type: 'text', value: r.name, maxlength: '100', 'aria-label': 'Name des Gerichts' });
    nameInput.addEventListener('change', function () {
      var v = nameInput.value.trim();
      if (!v) { nameInput.value = r.name; return; }
      r.name = v;
      var title = document.querySelector('[data-recipe="' + r.id + '"] .recipe-title');
      if (title) title.textContent = v;
      save(r);
    });

    var rows = r.ingredients.map(function (g) { return renderIngredient(r, g); });
    var confirming = state.confirmDelete === r.id;

    return h('div', { class: 'recipe-body' },
      h('label', { class: 'field' }, h('span', { class: 'field-label' }, 'Name'), nameInput),
      r.ingredients.length > 0 && h('div', { class: 'ing-row ing-head', 'aria-hidden': 'true' },
        h('span', {}, 'Zutat'), h('span', {}, 'Anzahl'), h('span', {}, 'Menge (optional)'), h('span', {}, 'Abteilung'), h('span', {}), h('span', {})),
      rows,
      h('button', { type: 'button', class: 'dashed-btn', onclick: function () {
        r.ingredients.push({ name: '', quantity: 1, amount: '', category_id: null, pantry: false });
        render();
        var inputs = document.querySelectorAll('[data-recipe="' + r.id + '"] .ing-row:not(.ing-head) .ing-name-input');
        if (inputs.length) inputs[inputs.length - 1].focus();
      } }, '＋ Zutat'),
      confirming
        ? h('div', { class: 'confirm-strip' },
          h('p', {}, '„' + r.name + '“ löschen? Kommende Termine mit diesem Gericht bekommen ein anderes, Zutaten auf der Liste bleiben stehen.'),
          h('button', { type: 'button', class: 'btn btn-danger-soft', onclick: function () { remove(r); } }, 'Löschen'),
          h('button', { type: 'button', class: 'btn btn-ghost', onclick: function () { state.confirmDelete = null; render(); } }, 'Abbrechen'))
        : h('div', { class: 'recipe-actions' },
          h('button', { type: 'button', class: 'btn btn-danger-soft', onclick: function () { state.confirmDelete = r.id; render(); } }, 'Gericht löschen')));
  }

  function renderIngredient(r, g) {
    var name = h('input', { type: 'text', class: 'ing-name-input', value: g.name, maxlength: '500', placeholder: 'Zutat', 'aria-label': 'Zutat' });
    name.addEventListener('input', function () { g.name = name.value; scheduleSave(r); });

    var qty = h('input', { type: 'number', class: 'ing-qty', min: '1', max: '999', value: String(g.quantity), 'aria-label': 'Anzahl auf der Einkaufsliste' });
    qty.addEventListener('change', function () {
      g.quantity = Math.min(999, Math.max(1, parseInt(qty.value, 10) || 1));
      qty.value = g.quantity;
      scheduleSave(r);
    });

    var amount = h('input', { type: 'text', class: 'ing-amount', value: g.amount, maxlength: '50', placeholder: 'z. B. 500 g', 'aria-label': 'Menge, optional' });
    amount.addEventListener('input', function () { g.amount = amount.value; scheduleSave(r); });

    var cat = h('select', { class: 'ing-cat', 'aria-label': 'Abteilung' },
      h('option', { value: '' }, 'Ohne Kategorie'),
      state.categories.map(function (c) { return h('option', { value: String(c.id) }, c.name); }));
    cat.value = g.category_id != null ? String(g.category_id) : '';
    cat.addEventListener('change', function () { g.category_id = cat.value ? parseInt(cat.value, 10) : null; save(r); });

    var pantry = h('input', { type: 'checkbox', checked: g.pantry });
    pantry.addEventListener('change', function () { g.pantry = pantry.checked; save(r); });

    return h('div', { class: 'ing-row' }, name, qty, amount, cat,
      h('label', { class: 'pantry', title: 'Grundvorrat ist beim Übernehmen automatisch als „zu Hause“ abgehakt' }, pantry, 'Grundvorrat'),
      h('button', { type: 'button', class: 'btn-icon btn-danger ing-del', title: 'Zutat entfernen',
        'aria-label': (g.name || 'Zutat') + ' entfernen', onclick: function () {
          r.ingredients.splice(r.ingredients.indexOf(g), 1);
          render();
          save(r);
        } }, '🗑'));
  }

  async function remove(r) {
    try {
      await U.api('DELETE', '/api/recipes/' + r.id);
      state.recipes.splice(state.recipes.indexOf(r), 1);
      state.open = null;
      state.confirmDelete = null;
      render();
      U.toast('„' + r.name + '“ gelöscht');
    } catch (err) {
      console.error('delete recipe failed', err);
      U.toast('„' + r.name + '“ konnte nicht gelöscht werden.');
    }
  }

  addForm.addEventListener('submit', async function (e) {
    e.preventDefault();
    var name = addInput.value.trim();
    if (!name) return;
    try {
      var r = await U.api('POST', '/api/recipes', { name: name });
      state.recipes.push(r);
      state.recipes.sort(function (a, b) { return a.name.localeCompare(b.name, 'de'); });
      state.open = r.id;
      addInput.value = '';
      render();
      var first = document.querySelector('[data-recipe="' + r.id + '"] .dashed-btn');
      if (first) first.focus();
    } catch (err) {
      console.error('create recipe failed', err);
      U.toast('Das Gericht konnte nicht angelegt werden.');
    }
  });

  // Flush pending debounced saves when leaving the page.
  window.addEventListener('pagehide', function () {
    Object.keys(saveTimers).forEach(function (id) {
      var r = state.recipes.find(function (x) { return String(x.id) === id; });
      if (r) save(r);
    });
  });

  load();
})();
