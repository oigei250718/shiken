// 录入页快捷键：⌘+Enter（兼容 Ctrl+Enter）提交表单；普通回车不提交，直接忽略
document.addEventListener('keydown', function (e) {
  if (e.key !== 'Enter') return;
  var form = document.querySelector('form[data-quick-save]');
  if (!form || !form.contains(e.target)) return;
  if (e.metaKey || e.ctrlKey) {
    e.preventDefault();
    form.requestSubmit();
    return;
  }
  // 普通回车：在输入框 / 下拉框中按回车不做任何事（textarea 中的回车仍为换行）
  var tag = e.target.tagName;
  if (tag === 'INPUT' || tag === 'SELECT') e.preventDefault();
});

// 全选/取消全选
function toggleAll(master) {
  document.querySelectorAll('input[name="ids"]').forEach(function (cb) {
    cb.checked = master.checked;
  });
}

// 带行号的多行输入框：自动调整高度；超过上限出现滚动条并同步行号；初始内容过长时滚动到末尾
function initLinedTextarea(ta) {
  if (ta.getAttribute('data-lined') === '1') return;
  ta.setAttribute('data-lined', '1');

  var max = parseInt(ta.getAttribute('data-max-height') || '360', 10);
  var wrap = document.createElement('div');
  wrap.className = 'editor';
  ta.parentNode.insertBefore(wrap, ta);
  var gutter = document.createElement('div');
  gutter.className = 'editor-gutter';
  wrap.appendChild(gutter);
  wrap.appendChild(ta);

  function update() {
    // 行号 = 逻辑行数
    var lines = ta.value.split('\n').length;
    if (gutter.childElementCount !== lines) {
      var html = '';
      for (var i = 1; i <= lines; i++) html += '<span>' + i + '</span>';
      gutter.innerHTML = html;
    }
    // 自动高度：随内容增长，超过 max 后固定并出滚动条
    ta.style.height = 'auto';
    var h = Math.max(66, Math.min(ta.scrollHeight, max));
    ta.style.height = h + 'px';
    ta.style.overflowY = ta.scrollHeight > max ? 'auto' : 'hidden';
    gutter.style.height = h + 'px';
    gutter.scrollTop = ta.scrollTop;
  }

  ta.addEventListener('input', update);
  ta.addEventListener('scroll', function () {
    gutter.scrollTop = ta.scrollTop;
  });
  update();
  // 内容超出可视高度时，默认滚动展示末尾
  if (ta.scrollHeight > max) ta.scrollTop = ta.scrollHeight;
}

document.querySelectorAll('textarea.lined').forEach(initLinedTextarea);

// 动态含义块（含义 + 例句 textarea），单词/语法表单共用
function addMeaningBlock(containerId) {
  var c = document.getElementById(containerId);
  if (!c) return;
  var div = document.createElement('div');
  div.className = 'meaning-block';
  var row = document.createElement('div');
  row.className = 'dyn-row';
  var input = document.createElement('input');
  input.type = 'text';
  input.name = 'meaning_texts[]';
  input.placeholder = '含义';
  var btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'btn btn-small btn-danger';
  btn.textContent = '×';
  btn.onclick = function () { removeMeaningBlock(btn); };
  row.appendChild(input);
  row.appendChild(btn);
  var ta = document.createElement('textarea');
  ta.name = 'meaning_examples[]';
  ta.rows = 2;
  ta.className = 'lined';
  ta.setAttribute('data-max-height', '300');
  ta.setAttribute('wrap', 'off');
  ta.placeholder = '该含义的例句，一行一个';
  div.appendChild(row);
  div.appendChild(ta);
  c.appendChild(div);
  initLinedTextarea(ta);
  input.focus();
}
function addWordMeaningBlock() {
  addMeaningBlock('word-meanings');
}
function removeMeaningBlock(btn) {
  var block = btn.closest('.meaning-block');
  var container = block.parentElement;
  if (container.querySelectorAll('.meaning-block').length > 1) {
    block.remove();
  } else {
    block.querySelector('input').value = '';
    block.querySelector('textarea').value = '';
  }
}

// 关联选择器（单词/语法表单共用，通过 data-api 区分搜索接口）
function removeRelatedChip(btn) {
  btn.closest('.chip').remove();
}

(function initRelatedPicker() {
  var picker = document.getElementById('related-picker');
  if (!picker) return;
  var searchInput = document.getElementById('related-search');
  var resultsBox = document.getElementById('related-results');
  var chipsBox = document.getElementById('related-chips');
  var exclude = picker.getAttribute('data-exclude') || '0';
  var api = picker.getAttribute('data-api') || '/api/words/search';
  var timer = null;

  function selectedIDs() {
    var ids = {};
    chipsBox.querySelectorAll('input[name="related_ids[]"]').forEach(function (inp) {
      ids[inp.value] = true;
    });
    return ids;
  }

  function esc(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  function search() {
    var q = searchInput.value.trim();
    if (!q) {
      resultsBox.style.display = 'none';
      resultsBox.innerHTML = '';
      return;
    }
    fetch(api + '?q=' + encodeURIComponent(q) + '&exclude=' + encodeURIComponent(exclude))
      .then(function (res) { return res.json(); })
      .then(function (list) {
        var selected = selectedIDs();
        var html = '';
        var shown = 0;
        list.forEach(function (item) {
          if (selected[String(item.id)]) return;
          shown++;
          var label = item.word || item.format;
          if (item.kana) label += '（' + item.kana + '）';
          var sub = item.meanings && item.meanings.length ? item.meanings.join('；') : '';
          html += '<div class="rel-result" data-id="' + item.id + '" data-label="' + esc(label) + '">' +
            '<span class="jp">' + esc(label) + '</span>' +
            (sub ? '<span class="rel-result-sub">' + esc(sub) + '</span>' : '') +
            '</div>';
        });
        resultsBox.innerHTML = shown ? html : '<div class="rel-result-empty">无匹配结果</div>';
        resultsBox.style.display = 'block';
      });
  }

  searchInput.addEventListener('input', function () {
    clearTimeout(timer);
    timer = setTimeout(search, 250);
  });
  searchInput.addEventListener('focus', search);
  searchInput.addEventListener('keydown', function (e) {
    // 避免在搜索框按 ⌘+Enter 之外的回车误提交表单
    if (e.key === 'Enter' && !e.metaKey && !e.ctrlKey) e.preventDefault();
  });

  resultsBox.addEventListener('click', function (e) {
    var item = e.target.closest('.rel-result');
    if (!item) return;
    var chip = document.createElement('span');
    chip.className = 'chip';
    chip.setAttribute('data-id', item.getAttribute('data-id'));
    chip.innerHTML = '<span class="jp">' + esc(item.getAttribute('data-label')) + '</span>' +
      '<input type="hidden" name="related_ids[]" value="' + item.getAttribute('data-id') + '">' +
      '<button type="button" class="chip-remove" onclick="removeRelatedChip(this)">×</button>';
    chipsBox.appendChild(chip);
    searchInput.value = '';
    resultsBox.style.display = 'none';
    resultsBox.innerHTML = '';
    searchInput.focus();
  });

  document.addEventListener('click', function (e) {
    if (!picker.contains(e.target)) {
      resultsBox.style.display = 'none';
    }
  });
})();

// 单词测试
function initTest() {
  var words = window.TEST_WORDS || [];
  var mode = window.TEST_MODE || 'word';
  var idx = 0;
  var unknown = [];

  var modeHints = {
    word: '根据「单词」回忆假名与含义',
    kana: '根据「假名」回忆单词与含义',
    meaning: '根据「含义」回忆单词与假名'
  };
  document.getElementById('test-mode-hint').textContent = modeHints[mode] || '';

  function render() {
    var w = words[idx];
    var prompt;
    if (mode === 'kana') {
      prompt = w.kana || w.word;
    } else if (mode === 'meaning') {
      prompt = w.meanings.map(function (m) { return m.text; }).join('；');
    } else {
      prompt = w.word;
    }
    document.getElementById('test-prompt').textContent = prompt;
    document.getElementById('test-answer').style.display = 'none';
    document.getElementById('test-answer').innerHTML = '';
    document.getElementById('btn-detail').href = '/words/' + w.id;
    document.getElementById('test-counter').textContent = (idx + 1) + ' / ' + words.length;
    document.getElementById('progress-fill').style.width = (idx / words.length * 100) + '%';
  }

  function answer(known) {
    if (!known) unknown.push(words[idx].id);
    idx++;
    if (idx >= words.length) {
      finish();
    } else {
      render();
    }
  }

  function finish() {
    document.getElementById('progress-fill').style.width = '100%';
    fetch('/test/finish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ total: words.length, unknown: unknown })
    })
      .then(function (res) { return res.json(); })
      .then(function (data) { window.location.href = data.redirect; })
      .catch(function () {
        alert('提交结果失败，请重试');
        window.location.href = '/test';
      });
  }

  document.getElementById('btn-peek').onclick = function () {
    var w = words[idx];
    var el = document.getElementById('test-answer');
    var parts = [];
    if (mode !== 'word') parts.push('<div><strong>' + esc(w.word) + '</strong></div>');
    if (mode !== 'kana' && w.kana) parts.push('<div>假名：' + esc(w.kana) + '</div>');
    if (mode !== 'meaning') {
      parts.push('<div>含义：' + esc(w.meanings.map(function (m) { return m.text; }).join('；')) + '</div>');
    }
    w.meanings.forEach(function (m) {
      (m.examples || []).forEach(function (ex) {
        parts.push('<div class="test-example">例句：' + esc(ex) + '</div>');
      });
    });
    el.innerHTML = parts.join('');
    el.style.display = 'block';
  };
  document.getElementById('btn-known').onclick = function () { answer(true); };
  document.getElementById('btn-unknown').onclick = function () { answer(false); };

  function esc(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  render();
}
