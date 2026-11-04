package server

const panelHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Панель управления</title>
<style>
:root{--bg:#0f1014;--card:#1a1b21;--card2:#24252d;--line:#2e2f39;--text:#ececf1;--muted:#8b8d98;--accent:#f4b860;--ok:#4ade80;--bad:#f87171}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.wrap{max-width:1240px;margin:0 auto;padding:24px 20px 60px}
header{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:22px;flex-wrap:wrap}
.brand{display:flex;align-items:center;gap:12px}.mark{width:40px;height:40px;border-radius:12px;background:var(--accent);color:#1a1205;display:grid;place-items:center;font-weight:800;font-size:18px}
.brand h1{font-size:18px;margin:0}.brand span{color:var(--muted);font-size:13px}
button{font:inherit;border:0;border-radius:10px;padding:9px 14px;cursor:pointer;font-weight:600}
.primary{background:var(--accent);color:#1a1205}.ghost{background:var(--card2);color:var(--text)}.link{background:transparent;color:var(--muted);padding:6px 8px;font-weight:500}.link:hover{color:var(--text);background:var(--card2)}
.link.bad{color:var(--bad)}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:14px;margin-bottom:20px}
.stat{background:var(--card);border-radius:16px;padding:16px 18px}.stat b{display:block;font-size:22px;margin-top:4px}.stat span{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.04em}
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:20px}@media(max-width:900px){.grid2{grid-template-columns:1fr}}
.panel{background:var(--card);border-radius:18px;padding:20px;margin-bottom:20px}
.panel h2{font-size:16px;margin:0 0 14px;display:flex;justify-content:space-between;align-items:center;gap:10px}
.kv{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:10px}.kv div{background:var(--card2);border-radius:12px;padding:10px 12px}.kv span{color:var(--muted);display:block;font-size:12px}
code,.mono{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;word-break:break-all}
table{width:100%;border-collapse:collapse}th{color:var(--muted);font-weight:500;text-align:left;font-size:12px;padding:8px 8px;border-bottom:1px solid var(--line)}
td{padding:11px 8px;border-bottom:1px solid var(--line);vertical-align:top}tr:last-child td{border-bottom:0}
.dot{display:inline-block;width:9px;height:9px;border-radius:50%;background:#555;margin-right:8px}.dot.on{background:var(--ok);box-shadow:0 0 0 4px rgba(74,222,128,.15)}.dot.off{background:var(--bad)}
.muted{color:var(--muted)}.tag{display:inline-block;background:var(--card2);border-radius:6px;padding:2px 7px;font-size:12px;margin:0 4px 4px 0}.tag.bad{color:var(--bad)}.tag.acc{color:var(--accent)}
.empty{color:var(--muted);text-align:center;padding:22px}
.row{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.row.end{justify-content:flex-end}
.modal{position:fixed;inset:0;background:rgba(0,0,0,.6);display:none;align-items:center;justify-content:center;padding:20px;z-index:5}.modal.show{display:flex}
.dialog{background:var(--card);border-radius:20px;padding:24px;width:min(560px,100%);max-height:92vh;overflow:auto}.dialog h3{margin:0 0 12px}
label{color:var(--muted);font-size:12px}
input,textarea,select{width:100%;font:inherit;background:var(--card2);color:var(--text);border:1px solid var(--line);border-radius:10px;padding:10px 12px;margin:4px 0 12px}
textarea{font-family:ui-monospace,Menlo,monospace;font-size:12px;height:96px;resize:none}
.warn{background:rgba(244,184,96,.12);color:var(--accent);border-radius:10px;padding:10px 12px;font-size:13px;margin-bottom:12px}
.err{color:var(--bad);min-height:18px;font-size:13px}.qr{display:block;margin:0 auto 12px;width:240px;height:240px;border-radius:12px;background:#fff;padding:8px}
.route{display:flex;flex-direction:column;gap:6px;margin-bottom:12px}.route .hop{display:flex;gap:8px;align-items:center}.route .hop select{margin:0}
.list{display:flex;flex-direction:column;gap:8px}.item{display:flex;justify-content:space-between;align-items:center;background:var(--card2);border-radius:12px;padding:10px 12px;gap:8px}
@media(max-width:800px){.hide-sm{display:none}}
</style>
</head>
<body>
<div class="wrap">
<header><div class="brand"><div class="mark">V</div><div><h1>Veil · панель управления</h1><span id="cover"></span></div></div>
<div class="row"><button class="primary" id="add">+ Клиент</button><button class="ghost" id="logout">Выйти</button></div></header>
<div class="stats">
<div class="stat"><span>Клиентов</span><b id="s-clients">—</b></div><div class="stat"><span>Онлайн</span><b id="s-online">—</b></div>
<div class="stat"><span>Получено</span><b id="s-rx">—</b></div><div class="stat"><span>Отправлено</span><b id="s-tx">—</b></div><div class="stat"><span>Аптайм</span><b id="s-up">—</b></div>
</div>
<div class="panel"><h2>Клиенты</h2>
<table><thead><tr><th>Клиент</th><th>Маршрут</th><th>Подключение</th><th class="hide-sm">Рукопожатие</th><th>↓/↑</th><th></th></tr></thead><tbody id="rows"></tbody></table></div>
<div class="grid2">
<div class="panel"><h2>Каскад: следующие узлы<span class="row"><button class="ghost" id="optimize">Перестроить сейчас</button><button class="ghost" id="up-add">+ Узел</button></span></h2><p class="muted">Узлы, через которые этот сервер может отправлять клиентов дальше. Маршрут может проходить через любое число узлов.</p><div class="list" id="ups"></div></div>
<div class="panel"><h2>Каскад: выданные ключи<button class="ghost" id="rel-add">+ Ключ</button></h2><p class="muted">Ключ разрешает другому узлу отправлять клиентов через этот сервер.</p><div class="list" id="rels"></div></div>
</div>
<div class="grid2">
<div class="panel"><h2>Сервер</h2><div class="kv">
<div><span>IPv6</span><code id="v6">—</code></div><div><span>IPv4</span><code id="v4">—</code></div><div><span>Транспорт</span><code>HTTP/3 → HTTP/2</code></div><div><span>Версия</span><code id="ver">—</code></div></div>
<p></p><label>Домен (откроет сайт-прикрытие и даст сертификат Let's Encrypt)</label>
<div class="row"><input id="domain" placeholder="example.com" style="flex:1;margin:0"><button class="ghost" id="domain-save">Сохранить</button></div>
<p class="muted" id="cert"></p></div>
<div class="panel"><h2>Пароль администратора</h2>
<label>Текущий пароль</label><input id="pw-cur" type="password" autocomplete="current-password">
<label>Новый пароль (не короче 8 символов)</label><input id="pw-new" type="password" autocomplete="new-password">
<label>Повторите новый пароль</label><input id="pw-rep" type="password" autocomplete="new-password">
<div class="err" id="pw-err"></div><div class="row end"><button class="primary" id="pw-save">Сменить пароль</button></div></div>
</div>
</div>

<div class="modal" id="m-client"><div class="dialog"><h3>Новый клиент</h3>
<label>Имя</label><input id="c-name" maxlength="40" autocomplete="off">
<label>Заметка</label><input id="c-note" maxlength="200" autocomplete="off">
<label>Маршрут</label><select id="c-mode"><option value="manual">Вручную — выбрать узлы по порядку</option><option value="auto">Автоматически по задержкам</option></select>
<div id="c-manual"><div class="route" id="c-route"></div></div>
<div id="c-auto" style="display:none"><label>Выходной узел</label><select id="c-exit"></select><label>Промежуточных узлов</label><select id="c-hops"></select>
<p class="muted">Вход и выход фиксированы, промежуточные узлы упорядочиваются по минимальной суммарной задержке и перестраиваются автоматически, когда другой путь стабильно быстрее на 20%. Конфиг клиента при этом не меняется.</p></div>
<div class="err" id="c-err"></div><div class="row end"><button class="ghost" data-close>Отмена</button><button class="primary" id="c-create">Создать</button></div></div></div>

<div class="modal" id="m-edit"><div class="dialog"><h3>Клиент</h3>
<label>Имя</label><input id="e-name" maxlength="40"><label>Заметка</label><input id="e-note" maxlength="200">
<div class="err" id="e-err"></div><div class="row end"><button class="ghost" data-close>Отмена</button><button class="primary" id="e-save">Сохранить</button></div></div></div>

<div class="modal" id="m-up"><div class="dialog"><h3>Следующий узел каскада</h3>
<label>Название узла</label><input id="u-name" maxlength="40"><label>Ключ каскада (veil-relay://…), выданный тем узлом</label><textarea id="u-link"></textarea>
<div class="err" id="u-err"></div><div class="row end"><button class="ghost" data-close>Отмена</button><button class="primary" id="u-save">Подключить</button></div></div></div>

<div class="modal" id="m-rel"><div class="dialog"><h3>Новый ключ каскада</h3>
<label>Для какого узла (название)</label><input id="r-name" maxlength="40">
<div class="err" id="r-err"></div><div class="row end"><button class="ghost" data-close>Отмена</button><button class="primary" id="r-save">Выдать</button></div></div></div>

<div class="modal" id="m-link"><div class="dialog"><h3 id="l-title">Конфигурация</h3>
<div class="warn" id="l-warn"></div><img class="qr" id="l-qr" alt=""><p class="muted" id="l-via"></p><textarea id="l-text" readonly></textarea>
<div class="row end"><button class="ghost" id="l-copy">Скопировать</button><button class="ghost" id="l-dl">Скачать файл</button><button class="primary" data-close>Готово</button></div></div></div>

<script>
const H={'X-Panel-Request':'1','Content-Type':'application/json'};const $=id=>document.getElementById(id);let state=null,editing='',dlName='client.veil';
function bytes(n){const u=['Б','КБ','МБ','ГБ','ТБ'];let i=0;while(n>=1024&&i<u.length-1){n/=1024;i++}return (i?n.toFixed(1):n)+' '+u[i]}
function ago(t){if(!t)return '—';const s=Math.max(0,(Date.now()-new Date(t))/1000);if(s<60)return Math.floor(s)+' с назад';if(s<3600)return Math.floor(s/60)+' мин назад';if(s<86400)return Math.floor(s/3600)+' ч назад';return Math.floor(s/86400)+' дн назад'}
function dur(t){const s=(Date.now()-new Date(t))/1000;const d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60);return (d?d+' д ':'')+h+' ч '+m+' мин'}
function el(tag,cls,text){const e=document.createElement(tag);if(cls)e.className=cls;if(text!==undefined)e.textContent=text;return e}
function btn(text,cls,fn){const b=el('button',cls,text);b.onclick=fn;return b}
async function api(path,body){const r=await fetch('/admin/api/'+path,body?{method:'POST',headers:H,body:JSON.stringify(body)}:{headers:H});if(r.status===404&&path==='state'){location.reload();throw new Error('session')}let j={};try{j=await r.json()}catch(e){}if(!r.ok)throw new Error(j.error||('Ошибка '+r.status));return j}
function open(id){$(id).classList.add('show')}function close(id){$(id).classList.remove('show')}
document.querySelectorAll('[data-close]').forEach(b=>b.onclick=()=>b.closest('.modal').classList.remove('show'));
function showLink(title,r,warn){$('l-title').textContent=title;$('l-warn').textContent=warn;$('l-text').value=r.link;$('l-qr').src=r.qr||'';$('l-qr').style.display=r.qr?'block':'none';$('l-via').textContent=r.via?('Маршрут: '+r.via):'';dlName=r.file||((r.name||'relay')+'.txt');open('m-link')}
async function refresh(){try{state=await api('state')}catch(e){return}render()}
function render(){const s=state.server;$('cover').textContent='Сайт-прикрытие: '+s.company+(s.domain?' · '+s.domain:'');
$('s-clients').textContent=s.clients;$('s-online').textContent=s.online;$('s-rx').textContent=bytes(s.rx_bytes);$('s-tx').textContent=bytes(s.tx_bytes);$('s-up').textContent=dur(s.started);
$('v6').textContent=s.public_ipv6?('['+s.public_ipv6+']:'+s.port):'—';$('v4').textContent=s.public_ipv4?(s.public_ipv4+':'+s.port):'—';$('ver').textContent=s.version;
if(document.activeElement!==$('domain'))$('domain').value=s.domain||'';
const cs={none:'Домен не задан — клиенты подключаются по IP с закреплённым сертификатом.',pending:'Получаем сертификат…',ready:'Сертификат действителен до '+(s.cert.expires?new Date(s.cert.expires).toLocaleDateString('ru-RU'):'—')+'. Новые клиенты подключаются по домену.',error:s.cert.message||'Ошибка'};$('cert').textContent=cs[s.cert.state]||'';
const rows=$('rows');rows.replaceChildren();const own=state.clients.filter(c=>!c.relay_of),transit=state.clients.filter(c=>c.relay_of);
if(!own.length&&!transit.length){const tr=el('tr');const td=el('td','empty','Клиентов пока нет');td.colSpan=6;tr.append(td);rows.append(tr)}
for(const c of own.concat(transit)){const tr=el('tr');const n=el('td');n.append(el('span','dot'+(c.disabled?' off':c.online?' on':'')),el('strong','',c.name));if(c.note)n.append(el('div','muted',c.note));if(c.disabled)n.append(el('div','tag bad','заблокирован'));tr.append(n);
const r=el('td');if(c.auto)r.append(el('span','tag acc','авто → '+c.auto.exit+(c.auto.cost_ms>0?' · '+c.auto.cost_ms.toFixed(0)+' мс':c.auto.cost_ms<0?' · путь недоступен':'')));if(c.relay_of)r.append(el('span','tag acc','транзит от '+c.relay_of));if(c.route.length)r.append(el('span','tag','→ '+c.route.join(' → ')));else if(!c.relay_of)r.append(el('span','muted','выход здесь'));r.append(el('div','mono muted hide-sm',c.address4));tr.append(r);
const conn=el('td');if(c.sessions.length){for(const x of c.sessions){const d=el('div');d.append(el('span','tag',x.transport),el('span','mono muted',x.remote));conn.append(d)}}else conn.append(el('span','muted','—'));tr.append(conn);
tr.append(el('td','muted hide-sm',ago(c.last_handshake)));tr.append(el('td','mono',bytes(c.tx_bytes)+' / '+bytes(c.rx_bytes)));
const act=el('td');const box=el('div','row end');if(!c.relay_of){
box.append(btn('Изменить','link',()=>{editing=c.name;$('e-name').value=c.name;$('e-note').value=c.note||'';$('e-err').textContent='';open('m-edit')}));
box.append(btn(c.disabled?'Разблокировать':'Заблокировать','link',async()=>{try{await api('clients/update',{name:c.name,disabled:!c.disabled});refresh()}catch(e){alert(e.message)}}));
box.append(btn('Новый конфиг','link',async()=>{if(!confirm('Выпустить новый конфиг для «'+c.name+'»? Старый перестанет работать.'))return;try{const r=await api('clients/reissue',{name:c.name});showLink('Новый конфиг «'+c.name+'»',r,'Старый конфиг больше не работает. Ссылка показывается один раз.');refresh()}catch(e){alert(e.message)}}));
box.append(btn('Удалить','link bad',async()=>{if(!confirm('Удалить клиента «'+c.name+'»?'))return;try{await api('clients/delete',{name:c.name});refresh()}catch(e){alert(e.message)}}))}
act.append(box);tr.append(act);rows.append(tr)}
const ups=$('ups');ups.replaceChildren();if(!state.upstreams.length)ups.append(el('div','muted','Нет — этот сервер является выходом.'));
const lat={};for(const x of (state.latency||[]))lat[x.name]=x;
for(const u of state.upstreams){const it=el('div','item');const l=el('div');const x=lat[u.name];const ms=x?(x.ok?x.rtt_ms.toFixed(1)+' мс':'нет связи'):'замер…';l.append(el('strong','',u.name+' · '+ms),el('div','muted',u.company+' · '+(u.domain||u.ipv6||u.ipv4)+' · клиентов: '+u.clients));it.append(l,btn('Удалить','link bad',async()=>{if(!confirm('Отключить узел «'+u.name+'»?'))return;try{await api('upstreams/delete',{name:u.name});refresh()}catch(e){alert(e.message)}}));ups.append(it)}
const rels=$('rels');rels.replaceChildren();if(!state.relays.length)rels.append(el('div','muted','Ключи не выдавались.'));
for(const r of state.relays){const it=el('div','item');const l=el('div');l.append(el('strong','',r.name),el('div','muted',(r.online?'на связи':'не на связи')+' · клиентов: '+r.clients));it.append(l,btn('Отозвать','link bad',async()=>{if(!confirm('Отозвать ключ «'+r.name+'»? Все клиенты, идущие через этот узел, будут удалены.'))return;try{await api('relays/delete',{name:r.name});refresh()}catch(e){alert(e.message)}}));rels.append(it)}}
let route=[];
async function renderRoute(){const box=$('c-route');box.replaceChildren();
route.forEach((h,i)=>{const d=el('div','hop');d.append(el('span','tag',(i+1)+'. '+h));if(i===route.length-1)d.append(btn('убрать','link',()=>{route.pop();renderRoute()}));box.append(d)});
let opts=[];try{opts=(await api('explore',{route})).upstreams||[]}catch(e){box.append(el('div','err',e.message));return}
opts=opts.filter(o=>!route.includes(o));const d=el('div','hop');const sel=el('select');sel.append(new Option(route.length?'выход на последнем узле':'выход на этом сервере',''));for(const o of opts)sel.append(new Option('дальше через '+o,o));
sel.onchange=()=>{if(sel.value){route.push(sel.value);renderRoute()}};if(opts.length)d.append(sel);else d.append(el('span','muted',route.length?'дальше узлов нет — выход на последнем':'каскад не настроен — выход на этом сервере'));box.append(d)}
function syncMode(){const auto=$('c-mode').value==='auto';$('c-auto').style.display=auto?'block':'none';$('c-manual').style.display=auto?'none':'block'}
$('c-mode').onchange=syncMode;
$('add').onclick=()=>{$('c-name').value='';$('c-note').value='';$('c-err').textContent='';route=[];renderRoute();
const ex=$('c-exit');ex.replaceChildren();for(const u of state.upstreams)ex.append(new Option(u.name+' ('+u.company+')',u.name));
const hp=$('c-hops');hp.replaceChildren();hp.append(new Option('все доступные','-1'));for(let i=0;i<state.upstreams.length;i++)hp.append(new Option(String(i),String(i)));
$('c-mode').value='manual';$('c-mode').disabled=!state.upstreams.length;syncMode();open('m-client');$('c-name').focus()};
$('optimize').onclick=async()=>{$('optimize').disabled=true;try{const r=await api('cascade/optimize',{});const ev=r.events||[];alert(ev.length?ev.map(e=>e.error?(e.client+': '+e.error):(e.client+': '+e.from+' → '+e.to+' ('+(e.old_ms<0?'путь недоступен':e.old_ms.toFixed(0)+' мс')+' → '+e.new_ms.toFixed(0)+' мс)')).join('\n'):'Текущие маршруты оптимальны (или новый путь ещё не подтвердился — нужно два замера подряд).');refresh()}catch(e){alert(e.message)}finally{$('optimize').disabled=false}};
$('c-create').onclick=async()=>{$('c-create').disabled=true;try{const auto=$('c-mode').value==='auto';const body=auto?{name:$('c-name').value,note:$('c-note').value,auto:true,exit:$('c-exit').value,max_hops:parseInt($('c-hops').value,10)}:{name:$('c-name').value,note:$('c-note').value,route};const r=await api('clients',body);close('m-client');showLink('Клиент «'+r.name+'» создан',r,'Ссылка и QR показываются один раз: в них приватный ключ. Отсканируйте QR в приложении Veil или передайте файл.');refresh()}catch(e){$('c-err').textContent=e.message}finally{$('c-create').disabled=false}};
$('e-save').onclick=async()=>{try{await api('clients/update',{name:editing,new_name:$('e-name').value,note:$('e-note').value});close('m-edit');refresh()}catch(e){$('e-err').textContent=e.message}};
$('up-add').onclick=()=>{$('u-name').value='';$('u-link').value='';$('u-err').textContent='';open('m-up')};
$('u-save').onclick=async()=>{$('u-save').disabled=true;try{await api('upstreams',{name:$('u-name').value,link:$('u-link').value});close('m-up');refresh()}catch(e){$('u-err').textContent=e.message}finally{$('u-save').disabled=false}};
$('rel-add').onclick=()=>{$('r-name').value='';$('r-err').textContent='';open('m-rel')};
$('r-save').onclick=async()=>{try{const r=await api('relays',{name:$('r-name').value});close('m-rel');showLink('Ключ каскада для «'+r.name+'»',r,'Вставьте этот ключ в панели узла «'+r.name+'»: Каскад → следующие узлы → + Узел.');refresh()}catch(e){$('r-err').textContent=e.message}};
$('domain-save').onclick=async()=>{try{await api('settings/domain',{domain:$('domain').value});refresh()}catch(e){alert(e.message)}};
$('pw-save').onclick=async()=>{$('pw-err').textContent='';if($('pw-new').value!==$('pw-rep').value){$('pw-err').textContent='Пароли не совпадают';return}
try{await api('settings/password',{current:$('pw-cur').value,new:$('pw-new').value});alert('Пароль изменён. Войдите заново.');location.href='/admin'}catch(e){$('pw-err').textContent=e.message}};
$('l-copy').onclick=async()=>{try{await navigator.clipboard.writeText($('l-text').value);$('l-copy').textContent='Скопировано'}catch(e){$('l-text').select()}setTimeout(()=>$('l-copy').textContent='Скопировать',1500)};
$('l-dl').onclick=()=>{const a=document.createElement('a');a.href=URL.createObjectURL(new Blob([$('l-text').value+'\n'],{type:'application/octet-stream'}));a.download=dlName;a.click();URL.revokeObjectURL(a.href)};
$('logout').onclick=async()=>{try{await api('../logout',{})}catch(e){}location.href='/admin'};
refresh();setInterval(()=>{if(!document.querySelector('.modal.show'))refresh()},3000);
</script>
</body></html>`
