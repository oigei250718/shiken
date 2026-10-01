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

// 防重复提交：录入 / 编辑页的保存按钮（单词 · 语法 · 文章）
// 统一在 submit 事件上拦截，因此 ⌘+Enter 快捷保存（内部走 requestSubmit）同样被覆盖。
// 浏览器原生 required 校验不通过时不会派发 submit，故不会误锁按钮。
(function preventDoubleSubmit() {
  document.querySelectorAll('form[data-quick-save]').forEach(function (form) {
    var submitting = false;
    var btn = form.querySelector('.form-actions button[type="submit"]');
    form.addEventListener('submit', function (e) {
      if (submitting) {
        e.preventDefault(); // 连点：丢弃后续提交，避免重复写入
        return;
      }
      submitting = true;
      if (btn) {
        btn.disabled = true;
        btn.textContent = '保存中…';
      }
      // 兜底：若因后端报错等原因页面始终没有跳转，超时后解锁，避免按钮永久卡死
      setTimeout(function () {
        submitting = false;
        if (btn) {
          btn.disabled = false;
          btn.textContent = '保存';
        }
      }, 8000);
    });
  });
})();

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

// 录入单词联想：随输入实时匹配词库，点击候选项直接进入该单词的编辑页
// 仅新建页启用（模板侧用 Word.ID == 0 控制 data-word-suggest），编辑页不参与，避免改词时误跳。
(function initWordSuggest() {
  var input = document.getElementById('word-input');
  if (!input || !input.hasAttribute('data-word-suggest')) return;
  var box = document.getElementById('word-suggest');
  if (!box) return;

  var timer = null;
  var active = -1;

  function rows() { return box.querySelectorAll('.ac-item'); }

  function hide() {
    box.style.display = 'none';
    box.innerHTML = '';
    active = -1;
  }

  function highlight(idx) {
    var els = rows();
    if (!els.length) return;
    if (idx < 0) idx = els.length - 1;
    if (idx >= els.length) idx = 0;
    for (var i = 0; i < els.length; i++) els[i].classList.toggle('is-active', i === idx);
    active = idx;
    els[idx].scrollIntoView({ block: 'nearest' });
  }

  function gotoEdit(id) {
    window.location.href = '/words/' + id + '/edit';
  }

  function search() {
    var q = input.value.trim();
    if (!q) { hide(); return; }
    fetch('/api/words/search?q=' + encodeURIComponent(q))
      .then(function (res) { return res.json(); })
      .then(function (list) {
        active = -1;
        // 无匹配：不展示下拉，保持输入框安静，用户直接继续录入
        if (!list || !list.length) { hide(); return; }
        var html = '';
        list.slice(0, 8).forEach(function (item) {
          var label = item.word || '';
          if (item.kana) label += '（' + item.kana + '）';
          var sub = item.meanings && item.meanings.length ? item.meanings.join('；') : '';
          html += '<div class="ac-item" data-id="' + item.id + '">' +
            '<span class="ac-item-main"><span class="jp">' + esc(label) + '</span>' +
            '<span class="ac-item-tag">已在词库</span></span>' +
            (sub ? '<span class="ac-item-sub">' + esc(sub) + '</span>' : '') +
            '</div>';
        });
        html += '<div class="ac-tip">↑↓ 选择 · Enter 进入编辑 · Esc 关闭（⌘+Enter 仍为保存）</div>';
        box.innerHTML = html;
        box.style.display = 'block';
      })
      .catch(function () { hide(); });
  }

  input.addEventListener('input', function () {
    clearTimeout(timer);
    timer = setTimeout(search, 200);
  });

  input.addEventListener('keydown', function (e) {
    if (box.style.display !== 'block' || !rows().length) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      highlight(active + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      highlight(active - 1);
    } else if (e.key === 'Enter' && active >= 0 && !e.metaKey && !e.ctrlKey) {
      // 有高亮项时 Enter 是「进入编辑」；⌘/Ctrl+Enter 仍交给表单保存，不在此拦截
      e.preventDefault();
      gotoEdit(rows()[active].getAttribute('data-id'));
    } else if (e.key === 'Escape') {
      e.preventDefault();
      hide();
    }
  });

  // 用 mousedown 而非 click：早于输入框 blur 触发，避免失焦先把下拉关掉导致点击落空
  box.addEventListener('mousedown', function (e) {
    var el = e.target.closest('.ac-item');
    if (!el) return;
    e.preventDefault();
    gotoEdit(el.getAttribute('data-id'));
  });

  input.addEventListener('blur', function () { setTimeout(hide, 150); });
  input.addEventListener('focus', function () { if (input.value.trim()) search(); });

  function esc(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }
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
    if (w.isError) parts.push('<div class="test-error-flag"><span class="tag tag-error">易错单词</span></div>');
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
