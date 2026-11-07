package server

import (
	"net/http"
	"strings"

	"veil/internal/assets"
)

// Montserrat (OFL) из internal/assets отдаётся только авторизованному администратору: /admin/fonts/<файл>.
func serveAdminFont(w http.ResponseWriter, name string) {
	if name == "" || strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".ttf") {
		http.NotFound(w, nil)
		return
	}
	data, err := assets.Fonts.ReadFile("fonts/" + name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "font/ttf")
	h.Set("Cache-Control", "private, max-age=86400")
	h.Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

const panelHTML = `<!doctype html>
<html lang="ru" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Veil</title>
<script>
(function(){var th='',lg='';try{th=localStorage.getItem('veil-theme')||'';lg=localStorage.getItem('veil-lang')||''}catch(e){}
if(th!=='light'&&th!=='dark')th=(window.matchMedia&&matchMedia('(prefers-color-scheme: light)').matches)?'light':'dark';
if(lg!=='ru'&&lg!=='en')lg=/^ru/i.test(navigator.language||'')?'ru':'en';
document.documentElement.dataset.theme=th;document.documentElement.lang=lg})();
</script>
<style>
@font-face{font-family:Montserrat;font-weight:400 500;font-display:swap;src:url(/admin/fonts/Montserrat-Medium.ttf) format("truetype")}
@font-face{font-family:Montserrat;font-weight:600;font-display:swap;src:url(/admin/fonts/Montserrat-SemiBold.ttf) format("truetype")}
@font-face{font-family:Montserrat;font-weight:700 800;font-display:swap;src:url(/admin/fonts/Montserrat-Bold.ttf) format("truetype")}
:root,:root[data-theme=dark]{--bg:#0B0B0C;--card:#1B1C1F;--card2:#2A2B30;--ink:#FFFFFF;--ink2:#A8AAB2;--acc:#FF393D;--onacc:#0B0B0C;--accInk:#FF393D;--ok:#7DE0A0;color-scheme:dark}
:root[data-theme=light]{--bg:#FFFFFF;--card:#F2F3F5;--card2:#E4E6EA;--ink:#0B0B0C;--ink2:#5B6068;--acc:#FF393D;--onacc:#0B0B0C;--accInk:#DB1E32;--ok:#177A44;color-scheme:light}
*{box-sizing:border-box}
html,body{margin:0}
body{background:var(--bg);color:var(--ink);font-family:Montserrat,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;font-size:14px;line-height:1.5;-webkit-text-size-adjust:100%}
.num,code,.mono{font-family:Arimo,Arial,sans-serif}
button{font-family:inherit}
button:focus-visible,input:focus-visible,select:focus-visible,textarea:focus-visible{outline:2px solid var(--accInk);outline-offset:2px}
*{scrollbar-width:thin;scrollbar-color:var(--card2) transparent}
::-webkit-scrollbar{width:10px;height:10px;background:transparent}
::-webkit-scrollbar-thumb{background:var(--card2);border-radius:5px;border:3px solid transparent;background-clip:padding-box}
@keyframes aPop{from{opacity:0;transform:translateY(12px) scale(.98)}to{opacity:1;transform:none}}
@keyframes aFlow{to{stroke-dashoffset:-12}}
@keyframes aFade{from{opacity:0}to{opacity:1}}
@keyframes aRise{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
@keyframes aPulse{0%{box-shadow:0 0 0 0 rgba(125,224,160,.45)}100%{box-shadow:0 0 0 8px rgba(125,224,160,0)}}
@media (prefers-reduced-motion:reduce){*{animation-duration:.001s!important;transition-duration:.001s!important}}

.page{padding:24px}
.wrap{max-width:1240px;margin:0 auto}
header{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;margin-bottom:20px}
.brand{display:flex;align-items:center;gap:12px;min-width:0}
.brand h1{margin:0;font-size:18px;font-weight:600}
.brand small{font-size:13px;font-weight:500;color:var(--ink2)}
.hbtns{display:flex;gap:8px;flex-wrap:wrap}
.btn{min-height:44px;padding:0 18px;border:0;border-radius:14px;background:var(--card2);color:var(--ink);font-size:14px;font-weight:600;cursor:pointer}
.btn.acc{background:var(--acc);color:var(--onacc)}
.btn.sm{min-height:40px;padding:0 14px;border-radius:12px;font-size:13px}
.btn.acc.sm{padding:0 16px}
.btn:disabled{opacity:.55;cursor:default}
.btn.icon{padding:0;width:44px}
.lnk{border:0;background:transparent;color:var(--ink2);padding:6px 8px;border-radius:10px;font-size:13px;font-weight:500;cursor:pointer;min-height:36px}
.lnk:hover{background:var(--card2);color:var(--ink)}
.lnk.bad{color:var(--accInk)}

.stats{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px;margin-bottom:20px}
.stat{background:var(--card);border-radius:14px;padding:14px 16px;min-width:0}
.stat span{font-size:11px;font-weight:600;letter-spacing:.04em;color:var(--ink2);text-transform:uppercase}
.stat b{display:block;margin-top:6px;font-family:Arimo,Arial,sans-serif;font-size:22px;white-space:nowrap}

section.card{background:var(--card);border-radius:16px;padding:20px;margin-bottom:20px;min-width:0}
section.card h2{margin:0 0 6px;font-size:16px;font-weight:600;display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap}
section.card h2.m14{margin-bottom:14px}
.hint{margin:0 0 12px;font-size:13px;color:var(--ink2);line-height:1.5}
.btns{display:flex;gap:8px}
.grid2{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px;align-items:start}
.grid2 section.card{margin-bottom:20px}

.tag{display:inline-block;background:var(--card2);border-radius:6px;padding:2px 7px;font-size:12px;margin:0 4px 4px 0}
.tag.acc,.tag.bad{color:var(--accInk)}
.tag.bad{margin-top:6px}
.chips{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:12px}
.chips .tag{margin:0}
.chips .tag b{font-family:Arimo,Arial,sans-serif}
.dot{display:inline-block;flex:none;width:9px;height:9px;border-radius:50%;background:var(--ink2)}
.dot.on{background:var(--ok);animation:aPulse 1.6s ease-out infinite}
.dot.off{background:var(--accInk)}
.muted{color:var(--ink2)}

.gwrap{overflow:auto;background:var(--bg);border-radius:14px;padding:6px 0}
.gsize{flex:none;overflow:hidden}
.gcanvas{position:relative;transform-origin:0 0}
.gcanvas svg{position:absolute;left:0;top:0;overflow:visible}
.glabel{position:absolute;width:76px;text-align:center;background:var(--bg);border:1px solid var(--card2);border-radius:9px;padding:3px 0;font-size:11px;line-height:1.3;z-index:2}
.glabel b{font-family:Arimo,Arial,sans-serif}
.gnode{position:absolute;background:var(--card);border-radius:14px;border:2px solid var(--card2);box-shadow:0 4px 12px rgba(0,0,0,.18)}
.gnode.sel{border-color:var(--acc)}
.ghead{display:flex;align-items:center;gap:8px;width:100%;padding:10px 12px;border:0;border-radius:12px 12px 0 0;background:var(--card2);color:var(--ink);cursor:pointer;text-align:left}
.ghead strong{font-size:14px;flex:1;min-width:0;overflow:hidden;white-space:nowrap;text-overflow:ellipsis}
.ghead .role{flex:none;font-size:10px;font-weight:700;letter-spacing:.03em;text-transform:uppercase;color:var(--ink2)}
.ghead .role.key{color:var(--accInk)}
.grows{padding:8px 12px;display:flex;flex-direction:column;gap:4px;font-size:12px}
.grows div{display:flex;justify-content:space-between;gap:8px}
.grows span:first-child{color:var(--ink2)}
.grows span:last-child{font-family:Arimo,Arial,sans-serif;font-weight:700;text-align:right;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.pin{position:absolute;width:12px;height:12px;border-radius:3px;border:2px solid var(--bg)}
.pin.in{background:var(--acc)}.pin.out{background:var(--ink)}
.gctl{display:flex;gap:6px;padding:0 10px 10px}
.gctl button{flex:1;min-height:34px;border:0;border-radius:10px;background:var(--card2);color:var(--ink);font-size:14px;font-weight:600;cursor:pointer}
.gctl button.bad{flex:none;width:40px;color:var(--accInk)}
.legend{display:flex;flex-wrap:wrap;gap:14px;margin-top:12px;font-size:12px;color:var(--ink2)}
.legend span{display:inline-flex;align-items:center;gap:6px}
.lg{display:inline-block;width:22px;height:0}

body.fresh .stat,body.fresh .crow:not(.head),body.fresh .ccard{animation:aRise .35s both}
body.fresh .gnode{animation:aRise .4s both}
.ctable{display:block}.ccards{display:none}
.crow{display:grid;grid-template-columns:1.3fr 1.3fr 1.3fr .9fr .9fr 460px;gap:0 12px;padding:12px 8px;border-bottom:1px solid var(--card2);align-items:start}
.crow.head{font-size:12px;font-weight:500;color:var(--ink2);padding:0 8px 8px}
.crow.head .chs{font-size:12px}
.crow>div,.crow>span{min-width:0}
.cname{display:flex;align-items:center;gap:8px}.cname strong{font-size:14px;word-break:break-word}
.cnote{font-size:12px;color:var(--ink2);margin-top:2px}
.ctags{display:flex;flex-wrap:wrap;gap:4px}.ctags .tag{margin:0}
.csess{font-size:12px}.csess div{margin-bottom:2px}.csess .num{color:var(--ink2);word-break:break-all}
.chs{font-size:13px;color:var(--ink2)}
.ctr{font-family:Arimo,Arial,sans-serif;font-size:12px;line-height:1.5}
.cact{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:2px}
.crow .empty{grid-column:1/-1;text-align:center;color:var(--ink2);padding:6px}
.ccard{background:var(--card2);border-radius:14px;padding:12px 14px;display:flex;flex-direction:column;gap:8px}
.ccard+.ccard{margin-top:10px}
.ccard .top{display:flex;align-items:center;justify-content:space-between;gap:8px}
.ccard .top strong{font-size:15px}
.ccard .cact{justify-content:flex-start;gap:0}
.ccard .tag{background:var(--card)}

.list{display:flex;flex-direction:column;gap:8px}
.item{display:flex;justify-content:space-between;align-items:center;gap:8px;background:var(--card2);border-radius:14px;padding:10px 14px}
.item>div{min-width:0}.item strong{font-size:14px;word-break:break-word}.item div div{font-size:12px;color:var(--ink2)}
.kv{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin-bottom:16px}
.kv div{background:var(--card2);border-radius:14px;padding:10px 14px;min-width:0}
.kv span{display:block;font-size:12px;color:var(--ink2)}
.kv code{font-size:13px;font-weight:700;word-break:break-all}
label{font-size:12px;color:var(--ink2)}
.lrow{display:flex;justify-content:space-between;align-items:center;gap:8px}.lrow .lnk{min-height:0;padding:0 6px;font-size:12px;line-height:21px}
input,textarea,select{display:block;width:100%;font:inherit;font-size:14px;background:var(--card2);color:var(--ink);border:1px solid transparent;border-radius:12px;padding:10px 14px;margin:6px 0 14px;min-height:44px}
textarea{height:96px;resize:none;font-family:Arimo,Arial,sans-serif;font-size:12px}
select{appearance:none;-webkit-appearance:none;padding-right:36px;cursor:pointer}
.sel{position:relative}.sel::after{content:"\25BE";position:absolute;right:14px;top:6px;line-height:44px;color:var(--ink2);pointer-events:none}
.sel select{margin-bottom:14px}
.sel.flat select,.sel.flat::after{margin:0}
.sel.flat::after{top:0}
.drow{display:flex;gap:8px;margin-top:6px}.drow input{margin:0;flex:1}
.err{color:var(--accInk);font-size:13px;margin-bottom:6px}.err:empty{display:none}
.end{display:flex;justify-content:flex-end;gap:8px;flex-wrap:wrap}
.pwd .end{margin-top:6px}

.modal{position:fixed;left:0;top:0;right:0;bottom:0;background:rgba(0,0,0,.6);display:none;align-items:center;justify-content:center;padding:16px;z-index:10;animation:aFade .25s both}
.modal.show{display:flex}
.dialog{background:var(--card);border-radius:16px;padding:24px;width:min(560px,100%);max-height:100%;overflow:auto;box-shadow:0 4px 12px rgba(0,0,0,.18);animation:aPop .3s cubic-bezier(.2,0,.2,1) both}
.dialog h3{margin:0 0 14px;font-size:18px;font-weight:600}
.dialog .end{margin-top:6px}
.warn{background:var(--card2);color:var(--accInk);border-radius:12px;padding:10px 14px;font-size:13px;line-height:1.45;margin-bottom:12px}
.qr{display:block;margin:0 auto 12px;width:240px;height:240px;border-radius:14px;background:#fff;padding:10px}
.route{display:flex;flex-direction:column;gap:6px;margin-bottom:12px}
.hop{display:flex;gap:8px;align-items:center}.hop .sel{flex:1}.hop select{margin:0}.hop .tag{margin:0}
.via{margin:0 0 12px;font-size:13px;color:var(--ink2)}

.builder{position:fixed;left:0;top:0;right:0;bottom:0;background:var(--bg);display:none;flex-direction:column;z-index:6;animation:aFade .25s both}
.builder.show{display:flex}
.bbar{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;padding:14px 24px;border-bottom:1px solid var(--card2)}
.bbar h2{margin:0;font-size:18px;font-weight:600}.bbar span.s{font-size:12px;color:var(--ink2)}
.bbtns{display:flex;gap:8px;flex-wrap:wrap;align-items:center}
.seg{display:flex;background:var(--card);border-radius:12px;padding:3px}
.seg button{min-height:34px;padding:0 12px;border:0;border-radius:9px;font-size:12px;font-weight:600;cursor:pointer;background:transparent;color:var(--ink)}
.seg button.on{background:var(--acc);color:var(--onacc)}
.bbody{flex:1;min-height:0;display:flex;flex-direction:row;overflow:auto}
.bbody aside{flex:none;padding:14px;display:flex;flex-direction:column;gap:8px}
.bbody aside.pal{width:230px;border-right:1px solid var(--card2)}
.bbody aside.ins{width:270px;border-left:1px solid var(--card2);gap:10px}
.cap{font-size:11px;font-weight:700;letter-spacing:.04em;color:var(--ink2);text-transform:uppercase}
.pitem{display:flex;justify-content:space-between;align-items:center;gap:8px;background:var(--card);border-radius:12px;padding:8px 10px}
.pitem.off{opacity:.55}
.pitem strong{font-size:13px}.pitem div div{font-size:11px;color:var(--ink2);line-height:1.4;word-break:break-word}
.pitem button{flex:none;width:36px;min-height:36px;border:0;border-radius:10px;background:var(--acc);color:var(--onacc);font-size:18px;font-weight:700;cursor:pointer}
.pitem.off button{background:var(--card2);color:var(--ink2);cursor:not-allowed}
.canvas{flex:1;min-width:0;min-height:260px;overflow:auto;padding:16px;background-color:var(--bg);background-image:radial-gradient(var(--card2) 1.2px,transparent 1.2px);background-size:20px 20px}
.warns{margin-top:14px;display:flex;flex-direction:column;gap:6px}
.warns div{background:var(--card);border-radius:12px;padding:8px 12px;font-size:12px}
.warns .bad{color:var(--accInk)}.warns .ok{color:var(--ok)}.warns .info{color:var(--ink2)}
.ins strong.t{font-size:16px}
.irow{display:flex;justify-content:space-between;gap:8px;background:var(--card);border-radius:12px;padding:8px 12px;font-size:12px}
.irow span{color:var(--ink2)}.irow b{font-family:Arimo,Arial,sans-serif;text-align:right;word-break:break-all}
.ibox{background:var(--card);border-radius:12px;padding:10px 12px}
.ibox small{font-size:11px;color:var(--ink2)}
.bars{display:flex;align-items:flex-end;gap:3px;height:48px;margin-top:6px}
.bars div{flex:1;border-radius:2px;background:var(--ink2)}
.bars div.last{background:var(--acc)}.bars div.fail{background:var(--accInk)}
.rbox{font-size:12px;line-height:1.5}.rbox .k{color:var(--ink2)}.rbox .v{font-weight:600}.rbox .n{font-family:Arimo,Arial,sans-serif;margin-top:4px}

.tgt{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-bottom:16px}
.tgt.tight{margin-bottom:10px}
.tgt .cap{margin-right:4px}
.tbtn{display:inline-flex;align-items:center;gap:8px;min-height:36px;padding:0 14px;border:0;border-radius:10px;font-size:13px;font-weight:600;cursor:pointer;background:var(--card);color:var(--ink)}
.tbtn.on{background:var(--acc);color:var(--onacc)}
.tbtn .dot{width:8px;height:8px}
.tnote{background:var(--card);border-radius:12px;padding:10px 14px;margin-bottom:16px;font-size:13px;line-height:1.45;color:var(--ink2)}
.tdown{background:var(--card);border:1px dashed var(--accInk);border-radius:14px;padding:14px 16px;margin-bottom:16px;display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}
.tdown>div{min-width:0}.tdown strong{color:var(--accInk);font-size:14px}.tdown div div{font-size:13px;color:var(--ink2);line-height:1.45;margin-top:2px}
#pbody.dim{opacity:.45;pointer-events:none}
.bar{height:8px;border-radius:4px;background:var(--card2);overflow:hidden;margin-bottom:12px}
.bar i{display:block;height:100%;width:40%;background:var(--acc);border-radius:4px;animation:aInd 1.2s linear infinite}
@keyframes aInd{from{margin-left:-40%}to{margin-left:100%}}
.srmeta{font-size:12px;color:var(--ink2);margin-left:10px}
.toast{position:fixed;left:50%;bottom:24px;transform:translateX(-50%);background:var(--ink);color:var(--bg);border-radius:14px;padding:12px 18px;font-size:14px;font-weight:600;z-index:20;animation:aRise .25s both;max-width:calc(100% - 32px);white-space:pre-line;text-align:center;display:none}
.toast.show{display:block}

@media(max-width:899px){
.stats{grid-template-columns:repeat(3,minmax(0,1fr))}
.grid2{grid-template-columns:1fr}
.crow{grid-template-columns:1.3fr 1.2fr 1.3fr .9fr 260px}
.crow .chs{display:none}
.bbody{flex-direction:column}
.bbody aside.pal,.bbody aside.ins{width:100%;border:0}
.bbody aside.pal{border-bottom:1px solid var(--card2)}
.bbody aside.ins{border-top:1px solid var(--card2)}
}
@media(max-width:639px){
.page{padding:14px}
.brand h1{font-size:16px}
.stats{grid-template-columns:repeat(2,minmax(0,1fr))}
.ctable{display:none}.ccards{display:block}
.kv{grid-template-columns:1fr}
.bbar{padding:14px}
}
</style>
</head>
<body class="fresh">
<div class="page"><div class="wrap">

<header>
<div class="brand">
<svg width="40" height="40" viewBox="0 0 32 32" aria-hidden="true"><path d="M3 4h8l9 23h-8z" fill="#FF393D"></path><path d="M29 4h-8l-9 23h8z" fill="#6B6F78"></path><path d="M16 16.8L12 27h8z" fill="#D9DBE0"></path></svg>
<div style="min-width:0"><h1 data-i="title"></h1><small id="cover"></small></div>
</div>
<div class="hbtns">
<button type="button" class="btn icon" id="lang-btn"></button>
<button type="button" class="btn icon" id="theme-btn"></button>
<button type="button" class="btn acc" id="add">+ <span data-i="client"></span></button>
<button type="button" class="btn" id="logout" data-i="logout"></button>
</div>
</header>

<div class="tgt" id="tgt" style="display:none"><span class="cap" data-i="targetL"></span><span id="tgt-btns" style="display:contents"></span></div>
<div class="tnote" id="tnote" style="display:none"></div>
<div class="tdown" id="tdown" style="display:none"><div><strong id="td-t"></strong><div id="td-x"></div></div><button type="button" class="btn sm" id="td-retry" data-i="retry"></button></div>
<div id="pbody">

<div class="stats" id="stats"></div>

<section class="card">
<h2><span data-i="scheme"></span><button type="button" class="btn acc sm" id="open-builder" data-i="builder"></button></h2>
<div class="chips" id="g-sum"></div>
<div class="gwrap" id="g-wrap"><div class="gsize" id="g-size"><div class="gcanvas" id="g-canvas"></div></div></div>
<div class="legend" id="g-legend"></div>
</section>

<section class="card">
<h2 class="m14" data-i="clients"></h2>
<div class="ctable" id="ctable"></div>
<div class="ccards" id="ccards"></div>
</section>

<div class="grid2">
<section class="card">
<h2><span data-i="next"></span><span class="btns"><button type="button" class="btn sm" id="optimize" data-i="rebuild"></button><button type="button" class="btn sm" id="up-add">+ <span data-i="node"></span></button></span></h2>
<p class="hint" data-i="nextHint"></p>
<div class="list" id="ups"></div>
</section>
<section class="card">
<h2><span data-i="keys"></span><button type="button" class="btn sm" id="rel-add">+ <span data-i="key"></span></button></h2>
<p class="hint" data-i="keysHint"></p>
<div class="list" id="rels"></div>
</section>
</div>

<div class="grid2">
<section class="card">
<h2 class="m14" data-i="server"></h2>
<div class="kv">
<div><span>IPv6</span><code id="v6">—</code></div><div><span>IPv4</span><code id="v4">—</code></div>
<div><span data-i="transport"></span><code>HTTP/3 → HTTP/2</code></div><div><span data-i="version"></span><code id="ver">—</code></div>
</div>
<label data-i="domain"></label>
<div class="drow"><input id="domain" placeholder="example.com" autocomplete="off"><button type="button" class="btn sm" id="domain-save" data-i="save"></button></div>
<p class="hint" style="margin:10px 0 0" id="cert"></p>
<label data-i="frontLabel" style="margin-top:20px"></label>
<div class="sel"><select id="front-sel"></select></div>
<div class="drow" id="front-custom" style="display:none;margin:-4px 0 12px"><input id="front-domain" placeholder="example.ru" autocomplete="off"></div>
<div class="drow"><button type="button" class="btn sm acc" id="front-save" data-i="frontApply"></button></div>
<p class="hint" style="margin:10px 0 0" id="front-state"></p>
<p class="hint" style="margin:6px 0 0" id="front-note"></p>
</section>
<section class="card">
<h2 data-i="cover2"></h2>
<p class="hint" data-i="coverHint"></p>
<div style="margin-bottom:12px"><span class="tag" id="sr-status"></span><span class="srmeta" id="sr-meta" style="display:none"></span></div>
<div class="bar" id="sr-bar" style="display:none"><i></i></div>
<div class="warn" id="sr-err" style="display:none"></div>
<label data-i="srUrl"></label><input id="sr-url" placeholder="https://" autocomplete="off" spellcheck="false">
<label data-i="srRef"></label><input id="sr-ref" placeholder="main" autocomplete="off" spellcheck="false">
<label data-i="srDir"></label><input id="sr-dir" placeholder="public" autocomplete="off" spellcheck="false">
<div class="lrow"><label data-i="srToken"></label><button type="button" class="lnk bad" id="sr-clrtok" data-i="srClrTok" style="display:none"></button></div><input id="sr-token" type="password" autocomplete="new-password">
<div class="end"><button type="button" class="btn" id="sr-reset" data-i="srReset"></button><button type="button" class="btn acc" id="sr-update" data-i="srUpd"></button></div>
</section>
<section class="card pwd">
<h2 class="m14" data-i="password"></h2>
<label data-i="pwCur"></label><input id="pw-cur" type="password" autocomplete="current-password">
<label data-i="pwNew"></label><input id="pw-new" type="password" autocomplete="new-password">
<label data-i="pwRep"></label><input id="pw-rep" type="password" autocomplete="new-password">
<div class="err" id="pw-err"></div>
<div class="end"><button type="button" class="btn acc" id="pw-save" data-i="pwSave"></button></div>
</section>
</div>

</div>
</div></div>

<div class="modal" id="m-client"><div class="dialog" role="dialog"><h3 data-i="newClient"></h3>
<div class="warn" id="c-warn" style="display:none" data-i="newAdminWarn"></div>
<label data-i="name"></label><input id="c-name" maxlength="40" autocomplete="off">
<label data-i="roleLabel"></label><div class="sel"><select id="c-role"><option value="user" data-i="roleUser"></option><option value="admin" data-i="roleAdmin"></option></select></div>
<label data-i="note"></label><input id="c-note" maxlength="200" autocomplete="off">
<label data-i="route"></label>
<div class="sel"><select id="c-mode"><option value="manual" data-i="modeManualOpt"></option><option value="auto" data-i="modeAutoOpt"></option></select></div>
<div id="c-manual"><div class="route" id="c-route"></div></div>
<div id="c-auto" style="display:none"><label data-i="exitNode"></label><div class="sel"><select id="c-exit"></select></div><label data-i="hopsCount"></label><div class="sel"><select id="c-hops"></select></div>
<p class="hint" data-i="autoHint"></p></div>
<div class="err" id="c-err"></div><div class="end"><button type="button" class="btn" data-close data-i="cancel"></button><button type="button" class="btn acc" id="c-create" data-i="create"></button></div></div></div>

<div class="modal" id="m-edit"><div class="dialog" role="dialog"><h3 data-i="clientT"></h3>
<label data-i="name"></label><input id="e-name" maxlength="40"><label data-i="note"></label><input id="e-note" maxlength="200">
<label data-i="roleLabel"></label><div class="sel"><select id="e-role"><option value="user" data-i="roleUser"></option><option value="admin" data-i="roleAdmin"></option></select></div>
<div class="err" id="e-err"></div><div class="end"><button type="button" class="btn" data-close data-i="cancel"></button><button type="button" class="btn acc" id="e-save" data-i="save"></button></div></div></div>

<div class="modal" id="m-role"><div class="dialog" role="dialog"><h3 data-i="roleT"></h3>
<div class="warn" data-i="roleWarn"></div>
<p class="hint" id="ro-client"></p>
<label data-i="roleLabel"></label><div class="sel"><select id="ro-role"><option value="user" data-i="roleUser"></option><option value="admin" data-i="roleAdmin"></option></select></div>
<p class="hint" data-i="roleHint"></p>
<div class="err" id="ro-err"></div><div class="end"><button type="button" class="btn" data-close data-i="cancel"></button><button type="button" class="btn acc" id="ro-save" data-i="save"></button></div></div></div>

<div class="modal" id="m-up"><div class="dialog" role="dialog"><h3 data-i="upTitle"></h3>
<label data-i="nodeName"></label><input id="u-name" maxlength="40"><label data-i="upKey"></label><textarea id="u-link"></textarea>
<div class="err" id="u-err"></div><div class="end"><button type="button" class="btn" data-close data-i="cancel"></button><button type="button" class="btn acc" id="u-save" data-i="connect"></button></div></div></div>

<div class="modal" id="m-rel"><div class="dialog" role="dialog"><h3 data-i="relTitle"></h3>
<div class="warn" id="r-warn" style="display:none" data-i="relAdminWarn"></div>
<label data-i="relFor"></label><input id="r-name" maxlength="40">
<label data-i="relAdminL"></label><div class="sel"><select id="r-admin"><option value="0" data-i="no"></option><option value="1" data-i="yes"></option></select></div>
<div class="err" id="r-err"></div><div class="end"><button type="button" class="btn" data-close data-i="cancel"></button><button type="button" class="btn acc" id="r-save" data-i="issue"></button></div></div></div>

<div class="modal" id="m-link"><div class="dialog" role="dialog"><h3 id="l-title"></h3>
<div class="warn" id="l-warn"></div><img class="qr" id="l-qr" alt="QR"><p class="via" id="l-via"></p><textarea id="l-text" readonly></textarea>
<div class="end"><button type="button" class="btn" id="l-copy"></button><button type="button" class="btn" id="l-dl" data-i="download"></button><button type="button" class="btn acc" data-close data-i="done"></button></div></div></div>

<div class="builder" id="builder">
<div class="bbar">
<div style="min-width:0"><h2 data-i="builder"></h2><span class="s" data-i="builderHint"></span></div>
<div class="bbtns">
<div class="seg" id="b-modes"></div>
<button type="button" class="btn sm" id="b-check" data-i="check"></button>
<button type="button" class="btn acc sm" id="b-apply" data-i="apply"></button>
<button type="button" class="btn sm" id="b-close">✕</button>
</div></div>
<div class="bbody">
<aside class="pal"><span class="cap" data-i="available"></span><div id="b-pal" style="display:flex;flex-direction:column;gap:8px"></div><button type="button" class="btn sm" id="b-byKey" data-i="byKey"></button><button type="button" class="btn sm" id="b-issueKey" data-i="issueKey"></button></aside>
<div class="canvas"><div class="gsize" id="b-size"><div class="gcanvas" id="b-canvas"></div></div><div class="warns" id="b-warns"></div></div>
<aside class="ins"><span class="cap" data-i="inspector"></span><strong class="t" id="i-title"></strong>
<div id="i-rows" style="display:flex;flex-direction:column;gap:6px"></div>
<div class="ibox"><small id="i-histcap"></small><div class="bars" id="i-bars"></div></div>
<div class="ibox rbox"><div class="k" data-i="route"></div><div class="v" id="i-route"></div><div class="n" id="i-total"></div></div>
</aside>
</div></div>

<div class="toast" id="toast" role="status"></div>

<script>
const H={'X-Panel-Request':'1','Content-Type':'application/json'};const $=id=>document.getElementById(id);
let NODE='',state=null,editing='',dlName='client.veil',lang=document.documentElement.lang==='en'?'en':'ru',theme=document.documentElement.dataset.theme==='light'?'light':'dark';
const D={ru:{
title:'Veil · панель управления',cover:'Сайт-прикрытие: {0}',client:'Клиент',clients:'Клиенты',logout:'Выйти',
stClients:'Клиентов',stOnline:'Онлайн',stRx:'Получено',stTx:'Отправлено',stUp:'Аптайм',
scheme:'Схема каскада',builder:'Конструктор каскада',builderHint:'Вход и выход задаются явно. Двигайте узлы, добавляйте их из списка и применяйте.',
check:'Проверить маршрут',apply:'Применить',available:'Доступные узлы',inspector:'Узел',route:'Маршрут',
cClient:'Клиент',cRoute:'Маршрут',cConn:'Подключение',cHs:'Рукопожатие',blocked:'заблокирован',
next:'Каскад: следующие узлы',rebuild:'Перестроить сейчас',node:'Узел',remove:'Удалить',
nextHint:'Узлы, через которые этот сервер может отправлять клиентов дальше. Маршрут может проходить через несколько узлов.',
keys:'Каскад: выданные ключи',key:'Ключ',keysHint:'Ключ разрешает другому узлу отправлять клиентов через этот сервер.',revoke:'Отозвать',
server:'Сервер',transport:'Транспорт',version:'Версия',domain:'Домен (откроет сайт-прикрытие и даст сертификат Let\'s Encrypt)',save:'Сохранить',
password:'Пароль администратора',pwCur:'Текущий пароль',pwNew:'Новый пароль (не короче 8 символов)',pwRep:'Повторите новый пароль',pwSave:'Сменить пароль',
frontLabel:'Маскировка под чужой сайт',frontApply:'Применить',frontNone:'Без маскировки',frontOther:'Свой домен…',
frontOn:'Включена: посторонние соединения уходят на настоящий {0}, свои клиенты проходят в туннель.',frontOff:'Выключена: сервер отвечает как собственный сайт-прикрытие.',
frontConfirm:'Включить маскировку под {0}?\n\nКонфиги клиентов, выданные раньше, перестанут работать: выпустите их заново (в них нет ключа). QUIC (UDP/443) будет отключён. Ключи каскада, выданные этим сервером, тоже нужно выдать заново.',
frontDoneOn:'Маскировка включена',frontDoneOff:'Маскировка выключена',frontDomainNote:'При включённой маскировке сертификат домена не обновится: Let\'s Encrypt не пройдёт проверку.',frontBad:'Укажите доменное имя сайта, например ya.ru',
certNone:'Домен не задан — клиенты подключаются по IP с закреплённым сертификатом.',certPending:'Получаем сертификат…',certReady:'Сертификат действителен до {0}. Новые клиенты подключаются по домену.',certErr:'Ошибка',
noClients:'Клиентов пока нет',auto:'авто',transitFrom:'транзит от {0}',exitHere:'выход здесь',unreachable:'путь недоступен',ms:'мс',
edit:'Изменить',block:'Заблокировать',unblock:'Разблокировать',newCfg:'Новый конфиг',del:'Удалить',
agoS:'{0} с назад',agoM:'{0} мин назад',agoH:'{0} ч назад',agoD:'{0} дн назад',dd:'д',hh:'ч',mm:'мин',
units:['Б','КБ','МБ','ГБ','ТБ'],
noUps:'Нет — этот сервер является выходом.',noKeys:'Ключи не выдавались.',measuring:'замер…',noLink:'нет связи',clientsN:'клиентов: {0}',online:'на связи',offline:'не на связи',
confirmDelNode:'Отключить узел «{0}»?',confirmRevoke:'Отозвать ключ «{0}»? Все клиенты, идущие через этот узел, будут удалены.',confirmReissue:'Выпустить новый конфиг для «{0}»? Старый перестанет работать.',confirmDelClient:'Удалить клиента «{0}»?',
newClient:'Новый клиент',roleLabel:'Роль',roleUser:'Пользователь',roleAdmin:'Администратор',nodeThis:'Этот сервер',nodeLabel:'Узел каскада',name:'Имя',note:'Заметка',modeManualOpt:'Вручную — выбрать узлы по порядку',modeAutoOpt:'Автоматически по задержкам',exitNode:'Выходной узел',hopsCount:'Промежуточных узлов',allAvail:'все доступные',
autoHint:'Вход и выход фиксированы, промежуточные узлы упорядочиваются по минимальной суммарной задержке и перестраиваются автоматически, когда другой путь стабильно быстрее на 20%. Конфиг клиента при этом не меняется.',
cancel:'Отмена',create:'Создать',clientT:'Клиент',upTitle:'Следующий узел каскада',nodeName:'Название узла',upKey:'Ключ каскада (veil-relay://…), выданный тем узлом',connect:'Подключить',
relTitle:'Новый ключ каскада',relFor:'Для какого узла (название)',issue:'Выдать',copy:'Скопировать',copied:'Скопировано',download:'Скачать файл',done:'Готово',
undo:'убрать',exitOnLast:'выход на последнем узле',exitOnThis:'выход на этом сервере',nextThrough:'дальше через {0}',noMoreNodes:'дальше узлов нет — выход на последнем',noCascade:'каскад не настроен — выход на этом сервере',
newCfgTitle:'Новый конфиг «{0}»',newCfgWarn:'Старый конфиг больше не работает. Ссылка показывается один раз.',createdTitle:'Клиент «{0}» создан',createdWarn:'Ссылка и QR показываются один раз: в них приватный ключ. Отсканируйте QR в приложении Veil или передайте файл.',
relKeyTitle:'Ключ каскада для «{0}»',relKeyWarn:'Вставьте этот ключ в панели узла «{0}»: Каскад → следующие узлы → + Узел.',routeVia:'Маршрут: {0}',
optNone:'Текущие маршруты оптимальны (или новый путь ещё не подтвердился — нужно два замера подряд).',pwMismatch:'Пароли не совпадают',pwDone:'Пароль изменён. Войдите заново.',errN:'Ошибка {0}',
thisServer:'Этот сервер',internet:'Интернет',source:'источник',entry:'вход',exit:'выход',entryExit:'вход и выход',inter:'промежуточный',
address:'Адрес',latency:'Задержка',status:'Состояние',company:'Компания',addresses:'Адреса',via:'Выход через',
nodesN:'Узлов',firstHop:'До первого узла',total:'Итого',mode:'Режим',modeAuto:'авто по задержкам',modeManual:'вручную',viaClient:'Маршрут клиента',onlineL:'На связи',
lgOk:'исправная связь (до 100 мс), идёт трафик',lgSlow:'медленная связь (100–200 мс)',lgBad:'нет связи или больше 200 мс',lgUnk:'переход не измеряется сервером',
schemeNone:'Каскад не настроен: пока трафик клиентов выходит прямо с этого сервера.',
mManual:'Вручную',mAuto:'Авто по задержкам',moveEarlier:'Сдвинуть раньше',moveLater:'Сдвинуть позже',rmRoute:'Убрать из маршрута',addRoute:'Добавить в маршрут {0}',
role:'Роль',latFromThis:'Задержка от этого сервера',position:'Позиция',histCap:'Задержка этого сервера до узла, последние замеры',noHistory:'Замеров пока нет',
nodesShort:'узл.',routeTotalFirst:'до первого узла: {0}',routeTotalUnk:'задержка по цепочке не измеряется',
wMany:'Больше 5 узлов: задержка растёт с каждым переходом.',wAuto:'Вход и выход фиксированы. Промежуточные узлы упорядочиваются по минимальной задержке и перестраиваются, когда другой путь быстрее на 20% и на 5 мс два замера подряд.',wOff:'Нет связи с узлами: {0}.',wOk:'Маршрут корректен: все узлы на связи.',
checkOk:'Маршрут принят сервером, узлов: {0}',applyEmpty:'Добавьте хотя бы один узел',
byKey:'Узел по ключу…',issueKey:'Выдать ключ…',
themeDark:'Тёмная тема',themeLight:'Светлая тема',langSwitch:'English',
targetL:'Узел',nodeNote:'Панель узла «{0}» открыта через каскад. Изменения применяются к этому узлу; это разрешено, потому что узел выдал ключ с правом администрирования.',
downTitle:'Узел «{0}» не отвечает',downText:'Нет связи с узлом. Данные этого узла недоступны, пока связь не восстановится.',retry:'Повторить',stillDown:'Узел по-прежнему не отвечает',
tagAdmin:'Администратор',roleAct:'Роль',roleT:'Роль клиента',roleWarn:'Администратор получает полный доступ к админ-панели этого сервера. Меняйте роль только для тех, кому доверяете.',
roleHint:'Пользователь: только подключается к VPN. Администратор: также открывает админ-панель по адресу https://admin.',roleClient:'Клиент: {0}',
newAdminWarn:'Администратор получает полный доступ к админ-панели этого сервера.',
relAdminL:'Разрешить администрирование',yes:'Да',no:'Нет',relAdminWarn:'Внимание: узел получает полный доступ к админ-панели этого сервера: клиенты, каскад, ключи и пароль. Выдавайте ключ только своим серверам.',relAdminSub:'администрирование',
cover2:'Сайт-прикрытие',coverHint:'Сайт, который видят посторонние, можно брать из Git-репозитория вместо сгенерированного.',
srUrl:'Адрес репозитория',srRef:'Ветка',srDir:'Подкаталог',srToken:'Токен доступа',srUpd:'Обновить',srReset:'Сбросить на сгенерированный',
srOk:'Обновлён {0}',srBusy:'Идёт загрузка…',srErr:'Ошибка',srErrText:'Не удалось загрузить репозиторий: {0}',srNone:'Используется сгенерированный сайт',srWait:'Ещё не загружен',
srNeedUrl:'Укажите адрес репозитория',srResetDone:'Восстановлен сгенерированный сайт',srQueued:'Обновление запущено',srMeta:'коммит {0} · файлов: {1}',
srClrTok:'Сбросить токен',srClrConfirm:'Сбросить сохранённый токен доступа? Закрытый репозиторий перестанет загружаться, пока вы не укажете токен снова.',srClrDone:'Токен сброшен',
downSince:'Нет связи с {0}. Данные этого узла недоступны, пока связь не восстановится.'
},en:{
title:'Veil · control panel',cover:'Cover site: {0}',client:'Client',clients:'Clients',logout:'Sign out',
stClients:'Clients',stOnline:'Online',stRx:'Received',stTx:'Sent',stUp:'Uptime',
scheme:'Cascade scheme',builder:'Cascade builder',builderHint:'Entry and exit are chosen explicitly. Move nodes, add them from the list and apply.',
check:'Check the route',apply:'Apply',available:'Available nodes',inspector:'Node',route:'Route',
cClient:'Client',cRoute:'Route',cConn:'Connection',cHs:'Handshake',blocked:'blocked',
next:'Cascade: next nodes',rebuild:'Rebuild now',node:'Node',remove:'Remove',
nextHint:'Nodes this server can pass clients to. The route may go through several of them.',
keys:'Cascade: issued keys',key:'Key',keysHint:'A key lets another node send clients through this server.',revoke:'Revoke',
server:'Server',transport:'Transport',version:'Version',domain:'Domain (opens the cover site and issues a Let\'s Encrypt certificate)',save:'Save',
password:'Administrator password',pwCur:'Current password',pwNew:'New password (at least 8 characters)',pwRep:'Repeat the new password',pwSave:'Change password',
frontLabel:'Masquerade as another site',frontApply:'Apply',frontNone:'No masquerading',frontOther:'Other domain…',
frontOn:'On: strangers are sent to the real {0}, your clients get through to the tunnel.',frontOff:'Off: the server answers as its own cover site.',
frontConfirm:'Masquerade as {0}?\n\nClient configs issued earlier will stop working: issue them again (they have no key). QUIC (UDP/443) will be turned off. Cascade keys issued by this server must be issued again too.',
frontDoneOn:'Masquerading is on',frontDoneOff:'Masquerading is off',frontDomainNote:'While masquerading is on, the domain certificate will not renew: Let\'s Encrypt validation cannot pass.',frontBad:'Enter a site domain name, for example ya.ru',
certNone:'No domain set: clients connect by IP with a pinned certificate.',certPending:'Getting the certificate…',certReady:'The certificate is valid until {0}. New clients connect by domain.',certErr:'Error',
noClients:'No clients yet',auto:'auto',transitFrom:'transit from {0}',exitHere:'exit on this server',unreachable:'path unavailable',ms:'ms',
edit:'Edit',block:'Block',unblock:'Unblock',newCfg:'New config',del:'Delete',
agoS:'{0} s ago',agoM:'{0} min ago',agoH:'{0} h ago',agoD:'{0} d ago',dd:'d',hh:'h',mm:'min',
units:['B','KB','MB','GB','TB'],
noUps:'None: this server is the exit.',noKeys:'No keys issued.',measuring:'measuring…',noLink:'no connection',clientsN:'clients: {0}',online:'online',offline:'offline',
confirmDelNode:'Disconnect node "{0}"?',confirmRevoke:'Revoke key "{0}"? All clients going through this node will be deleted.',confirmReissue:'Issue a new config for "{0}"? The old one will stop working.',confirmDelClient:'Delete client "{0}"?',
newClient:'New client',roleLabel:'Role',roleUser:'User',roleAdmin:'Administrator',nodeThis:'This server',nodeLabel:'Cascade node',name:'Name',note:'Note',modeManualOpt:'Manual: choose the nodes in order',modeAutoOpt:'Automatic by latency',exitNode:'Exit node',hopsCount:'Intermediate nodes',allAvail:'all available',
autoHint:'Entry and exit are fixed, intermediate nodes are ordered by the lowest total latency and rebuilt automatically when another path is consistently 20% faster. The client config does not change.',
cancel:'Cancel',create:'Create',clientT:'Client',upTitle:'Next cascade node',nodeName:'Node name',upKey:'Cascade key (veil-relay://…) issued by that node',connect:'Connect',
relTitle:'New cascade key',relFor:'For which node (name)',issue:'Issue',copy:'Copy',copied:'Copied',download:'Download file',done:'Done',
undo:'remove',exitOnLast:'exit on the last node',exitOnThis:'exit on this server',nextThrough:'next through {0}',noMoreNodes:'no more nodes: exit on the last one',noCascade:'no cascade configured: exit on this server',
newCfgTitle:'New config for "{0}"',newCfgWarn:'The old config no longer works. The link is shown once.',createdTitle:'Client "{0}" created',createdWarn:'The link and QR are shown once and contain a private key. Scan the QR in the Veil app or pass the file on.',
relKeyTitle:'Cascade key for "{0}"',relKeyWarn:'Paste this key in the panel of node "{0}": Cascade → next nodes → + Node.',routeVia:'Route: {0}',
optNone:'Current routes are optimal (or the new path is not confirmed yet: two checks in a row are needed).',pwMismatch:'Passwords do not match',pwDone:'Password changed. Sign in again.',errN:'Error {0}',
thisServer:'This server',internet:'Internet',source:'source',entry:'entry',exit:'exit',entryExit:'entry and exit',inter:'intermediate',
address:'Address',latency:'Latency',status:'Status',company:'Company',addresses:'Addresses',via:'Exit via',
nodesN:'Nodes',firstHop:'To the first node',total:'Total',mode:'Mode',modeAuto:'auto by latency',modeManual:'manual',viaClient:'Client route',onlineL:'Online',
lgOk:'healthy link (under 100 ms), data flows',lgSlow:'slow link (100–200 ms)',lgBad:'no link or over 200 ms',lgUnk:'hop not measured by the server',
schemeNone:'No cascade configured: client traffic leaves directly from this server.',
mManual:'Manual',mAuto:'Auto by latency',moveEarlier:'Move earlier',moveLater:'Move later',rmRoute:'Remove from the route',addRoute:'Add to the route {0}',
role:'Role',latFromThis:'Latency from this server',position:'Position',histCap:'Latency from this server to the node, last checks',noHistory:'No checks yet',
nodesShort:'nodes',routeTotalFirst:'to the first node: {0}',routeTotalUnk:'chain latency is not measured',
wMany:'More than 5 nodes: the latency grows with every hop.',wAuto:'Entry and exit are fixed. Intermediate nodes are ordered by the lowest total latency and rebuilt when another path is faster by 20% and 5 ms twice in a row.',wOff:'No connection to nodes: {0}.',wOk:'The route is valid: all nodes are online.',
checkOk:'The server accepted the route, nodes: {0}',applyEmpty:'Add at least one node',
byKey:'Node by key…',issueKey:'Issue a key…',
themeDark:'Dark theme',themeLight:'Light theme',langSwitch:'Русский',
targetL:'Node',nodeNote:'The panel of node "{0}" is opened through the cascade. Changes apply to that node; this is allowed because the node issued an administration key.',
downTitle:'Node "{0}" is not responding',downText:'No link to the node. The data of this node is unavailable until the link is restored.',retry:'Retry',stillDown:'Still no answer from the node',
tagAdmin:'Administrator',roleAct:'Role',roleT:'Client role',roleWarn:'An administrator gets full access to the admin panel of this server. Change the role only for people you trust.',
roleHint:'User: connects to the VPN only. Administrator: also opens the admin panel through https://admin.',roleClient:'Client: {0}',
newAdminWarn:'An administrator gets full access to the admin panel of this server.',
relAdminL:'Allow administration',yes:'Yes',no:'No',relAdminWarn:'Warning: the node gets full access to the admin panel of this server: clients, cascade, keys and password. Issue the key only to servers you control.',relAdminSub:'administration',
cover2:'Cover site',coverHint:'Take the site that outsiders see from a Git repository instead of the generated one.',
srUrl:'Repository address',srRef:'Branch',srDir:'Subdirectory',srToken:'Access token',srUpd:'Update',srReset:'Reset to the generated site',
srOk:'Updated {0}',srBusy:'Loading…',srErr:'Error',srErrText:'Could not load the repository: {0}',srNone:'The generated site is in use',srWait:'Not loaded yet',
srNeedUrl:'Enter the repository address',srResetDone:'The generated site is restored',srQueued:'Update started',srMeta:'commit {0} · files: {1}',
srClrTok:'Reset token',srClrConfirm:'Reset the saved access token? A private repository will stop loading until you enter the token again.',srClrDone:'Token reset',
downSince:'No link since {0}. The data of this node is unavailable until the link is restored.'
}};
function t(k){let s=D[lang][k];if(s===undefined)s=D.ru[k];if(s===undefined)return k;if(typeof s!=='string')return s;for(let i=1;i<arguments.length;i++)s=s.split('{'+(i-1)+'}').join(arguments[i]);return s}
function store(k,v){try{localStorage.setItem(k,v)}catch(e){}}
function applyStatic(){document.documentElement.lang=lang;document.documentElement.dataset.theme=theme;document.title='Veil · '+(lang==='ru'?'панель':'admin');
document.querySelectorAll('[data-i]').forEach(e=>{e.textContent=t(e.dataset.i)});
$('lang-btn').textContent=lang==='ru'?'EN':'RU';$('lang-btn').title=t('langSwitch');$('lang-btn').setAttribute('aria-label',t('langSwitch'));
$('theme-btn').textContent=theme==='dark'?'☀':'☾';$('theme-btn').title=theme==='dark'?t('themeLight'):t('themeDark');$('theme-btn').setAttribute('aria-label',$('theme-btn').title);
$('l-copy').textContent=t('copy')}
function num(s){return lang==='ru'?String(s).replace('.',','):String(s)}
function bytes(n){const u=t('units');let i=0;while(n>=1024&&i<u.length-1){n/=1024;i++}return num(i?String(+n.toFixed(1)):String(Math.round(n)))+' '+u[i]}
function ago(x){if(!x)return '—';const s=Math.max(0,(Date.now()-new Date(x))/1000);if(s<60)return t('agoS',Math.floor(s));if(s<3600)return t('agoM',Math.floor(s/60));if(s<86400)return t('agoH',Math.floor(s/3600));return t('agoD',Math.floor(s/86400))}
function dur(x){const s=(Date.now()-new Date(x))/1000;const d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60);return (d?d+' '+t('dd')+' ':'')+h+' '+t('hh')+' '+m+' '+t('mm')}
function ms1(v,d){return num(v.toFixed(d||0))+' '+t('ms')}
function el(tag,cls,text){const e=document.createElement(tag);if(cls)e.className=cls;if(text!==undefined)e.textContent=text;return e}
function btn(text,cls,fn){const b=el('button',cls,text);b.type='button';b.onclick=fn;return b}
let toastT=0;function toast(msg,len){const e=$('toast');e.textContent=msg;e.classList.add('show');clearTimeout(toastT);toastT=setTimeout(()=>e.classList.remove('show'),len||2600)}
async function api(path,body){const r=await fetch('/admin/api/'+path+((NODE&&path!=='cascade/nodes')?'?node='+encodeURIComponent(NODE):''),body?{method:'POST',headers:H,body:JSON.stringify(body)}:{headers:H});if(r.status===404&&path==='state'&&!NODE){location.reload();throw new Error('session')}let j={};try{j=await r.json()}catch(e){}if(!r.ok)throw new Error(j.error||t('errN',r.status));return j}
function open(id){$(id).classList.add('show')}function close(id){$(id).classList.remove('show')}
document.querySelectorAll('[data-close]').forEach(b=>b.onclick=()=>b.closest('.modal').classList.remove('show'));
function showLink(title,r,warn){$('l-title').textContent=title;$('l-warn').textContent=warn;$('l-text').value=r.link;$('l-qr').src=r.qr||'';$('l-qr').style.display=r.qr?'block':'none';$('l-via').textContent=r.via?t('routeVia',r.via):'';$('l-via').style.display=r.via?'block':'none';dlName=r.file||((r.name||'relay')+'.txt');open('m-link')}
const hist={};
function latMap(){const m={};for(const x of (state&&state.latency||[]))m[x.name]=x;return m}
function pushHist(){const lat=latMap();for(const u of state.upstreams){const a=hist[u.name]||(hist[u.name]=[]);const x=lat[u.name];if(x)a.push(x.ok?x.rtt_ms:-1);while(a.length>16)a.shift()}}
let nodes=[],nodeDown={},nodeSince={},nodeErr='';
function nodeKey(n){return n.route.join('>')}
function nodeName(){const n=nodes.find(x=>nodeKey(x)===NODE);return n&&n.name||t('thisServer')}
async function refresh(){const k=NODE;let st;try{st=await api('state')}catch(e){if(k!==NODE)return;if(NODE){nodeDown[NODE]=true;if(!nodeSince[NODE])nodeSince[NODE]=new Date();nodeErr=e.message}renderTargets();return}
if(k!==NODE)return;delete nodeDown[k];delete nodeSince[k];nodeErr='';state=st;pushHist();render()}
function nodeBad(n){const k=nodeKey(n);if(nodeDown[k])return true;if(n.route.length===1&&state){const x=latMap()[n.route[0]];if(x&&!x.ok)return true}return false}
function renderTargets(){const box=$('tgt-btns');box.replaceChildren();$('tgt').style.display=nodes.length>1?'flex':'none';
nodes.forEach(n=>{const k=nodeKey(n);const b=el('button','tbtn'+(k===NODE?' on':''));b.type='button';const d=el('span','dot');d.style.background=nodeBad(n)?'var(--accInk)':'var(--ok)';b.append(d,document.createTextNode(n.name||t('nodeThis')));b.onclick=()=>{if(k===NODE)return;NODE=k;srDirty=false;renderTargets();refresh()};box.append(b)});
const down=!!(NODE&&nodeDown[NODE]);const note=$('tnote'),dn=$('tdown');
note.style.display=NODE&&!down?'block':'none';if(NODE&&!down)note.textContent=t('nodeNote',nodeName());
dn.style.display=down?'flex':'none';if(down){$('td-t').textContent=t('downTitle',nodeName());$('td-x').textContent=(nodeSince[NODE]?t('downSince',nodeSince[NODE].toLocaleTimeString(lang==='ru'?'ru-RU':'en-GB',{hour:'2-digit',minute:'2-digit'})):t('downText'))+(nodeErr?' ('+nodeErr+')':'')}
$('tgt').classList.toggle('tight',!!NODE);$('pbody').classList.toggle('dim',down)}
$('td-retry').onclick=async()=>{await refresh();if(NODE&&nodeDown[NODE])toast(t('stillDown'))};

function render(){const s=state.server;
$('cover').textContent=t('cover',s.company+(s.domain?' · '+s.domain:''));
const st=[['stClients',s.clients],['stOnline',s.online],['stRx',bytes(s.rx_bytes)],['stTx',bytes(s.tx_bytes)],['stUp',dur(s.started)]];
const box=$('stats');box.replaceChildren();st.forEach((x,i)=>{const d=el('div','stat');d.style.animationDelay=(i*0.04)+'s';d.append(el('span','',t(x[0])),el('b','',String(x[1])));box.append(d)});
$('v6').textContent=s.public_ipv6?('['+s.public_ipv6+']:'+s.port):'—';$('v4').textContent=s.public_ipv4?(s.public_ipv4+':'+s.port):'—';$('ver').textContent=s.version;
if(document.activeElement!==$('domain'))$('domain').value=s.domain||'';
const cs={none:t('certNone'),pending:t('certPending'),ready:t('certReady',s.cert.expires?new Date(s.cert.expires).toLocaleDateString(lang==='ru'?'ru-RU':'en-GB'):'—'),error:s.cert.message||t('certErr')};$('cert').textContent=cs[s.cert.state]||'';
renderFront(s);renderClients();renderLists();renderSite();renderTargets();renderScheme();if($('builder').classList.contains('show'))renderBuilder()}

function clientTags(c){const a=[];
if(c.role==='admin')a.push(el('span','tag acc',t('tagAdmin')));
if(c.auto){let x=t('auto')+' → '+c.auto.exit;if(c.auto.cost_ms>0)x+=' · '+ms1(c.auto.cost_ms);else if(c.auto.cost_ms<0)x+=' · '+t('unreachable');a.push(el('span','tag acc',x))}
if(c.relay_of)a.push(el('span','tag acc',t('transitFrom',c.relay_of)));
if(c.route.length)a.push(el('span','tag','→ '+c.route.join(' → ')));else if(!c.relay_of)a.push(el('span','tag',t('exitHere')));
return a}
function clientSess(c){const w=el('div','csess');if(c.sessions.length)for(const x of c.sessions){const d=el('div');d.append(el('span','tag',x.transport),el('span','num',x.remote));w.append(d)}else w.append(el('span','muted','—'));return w}
function clientActs(c){const box=el('div','cact');if(c.relay_of)return box;
box.append(btn(t('edit'),'lnk',()=>{editing=c.name;$('e-name').value=c.name;$('e-note').value=c.note||'';editRole=c.role==='admin'?'admin':'user';$('e-role').value=editRole;$('e-err').textContent='';open('m-edit')}));
box.append(btn(c.disabled?t('unblock'):t('block'),'lnk',async()=>{try{await api('clients/update',{name:c.name,disabled:!c.disabled});refresh()}catch(e){toast(e.message)}}));
box.append(btn(t('roleAct'),'lnk',()=>{editing=c.name;$('ro-client').textContent=t('roleClient',c.name);$('ro-role').value=c.role==='admin'?'admin':'user';$('ro-err').textContent='';open('m-role')}));
box.append(btn(t('newCfg'),'lnk',async()=>{if(!confirm(t('confirmReissue',c.name)))return;try{const r=await api('clients/reissue',{name:c.name});showLink(t('newCfgTitle',c.name),r,t('newCfgWarn'));refresh()}catch(e){toast(e.message)}}));
box.append(btn(t('del'),'lnk bad',async()=>{if(!confirm(t('confirmDelClient',c.name)))return;try{await api('clients/delete',{name:c.name});refresh()}catch(e){toast(e.message)}}));
return box}
function clientName(c,big){const w=el('div');const l=el('div','cname');l.append(el('span','dot'+(c.disabled?' off':c.online?' on':'')),el('strong','',c.name));w.append(l);if(c.note)w.append(el('div','cnote',c.note));return w}
function renderClients(){const tb=$('ctable'),cd=$('ccards');tb.replaceChildren();cd.replaceChildren();
const own=state.clients.filter(c=>!c.relay_of),transit=state.clients.filter(c=>c.relay_of),all=own.concat(transit);
const hd=el('div','crow head');[t('cClient'),t('cRoute'),t('cConn')].forEach(x=>hd.append(el('span','',x)));hd.append(el('span','chs',t('cHs')),el('span','','↓/↑'),el('span'));tb.append(hd);
if(!all.length){const e=el('div','crow');e.append(el('div','empty',t('noClients')));tb.append(e);cd.append(el('div','muted',t('noClients')));return}
all.forEach((c,i)=>{const tr=el('div','crow');tr.style.animationDelay=(0.1+i*0.05)+'s';
const n=clientName(c);if(c.disabled)n.append(el('span','tag bad',t('blocked')));tr.append(n);
const tg=el('div','ctags');clientTags(c).forEach(x=>tg.append(x));tr.append(tg);tr.append(clientSess(c));
tr.append(el('span','chs',ago(c.last_handshake)));tr.append(el('span','ctr',bytes(c.tx_bytes)+' / '+bytes(c.rx_bytes)));tr.append(clientActs(c));tb.append(tr);
const cc=el('div','ccard');cc.style.animationDelay=(0.1+i*0.05)+'s';const top=el('div','top');const l=el('div','cname');l.append(el('span','dot'+(c.disabled?' off':c.online?' on':'')),el('strong','',c.name));top.append(l,el('span','ctr muted',bytes(c.tx_bytes)+' / '+bytes(c.rx_bytes)));cc.append(top);
if(c.note)cc.append(el('div','cnote',c.note));
const tg2=el('div','ctags');clientTags(c).forEach(x=>tg2.append(x));if(c.disabled)tg2.append(el('span','tag bad',t('blocked')));cc.append(tg2);
if(c.sessions.length)cc.append(clientSess(c));cc.append(clientActs(c));cd.append(cc)})}

function renderLists(){const ups=$('ups');ups.replaceChildren();if(!state.upstreams.length)ups.append(el('div','hint',t('noUps')));
const lat=latMap();
for(const u of state.upstreams){const it=el('div','item');const l=el('div');const x=lat[u.name];const m=x?(x.ok?ms1(x.rtt_ms,1):t('noLink')):t('measuring');
l.append(el('strong','',u.name+' · '+m),el('div','',u.company+' · '+(u.domain||u.ipv6||u.ipv4)+' · '+t('clientsN',u.clients)));
it.append(l,btn(t('remove'),'lnk bad',async()=>{if(!confirm(t('confirmDelNode',u.name)))return;try{await api('upstreams/delete',{name:u.name});refresh()}catch(e){toast(e.message)}}));ups.append(it)}
const rels=$('rels');rels.replaceChildren();if(!state.relays.length)rels.append(el('div','hint',t('noKeys')));
for(const r of state.relays){const it=el('div','item');const l=el('div');l.append(el('strong','',r.name),el('div','',(r.online?t('online'):t('offline'))+' · '+t('clientsN',r.clients)+(r.admin?' · '+t('relAdminSub'):'')));
it.append(l,btn(t('revoke'),'lnk bad',async()=>{if(!confirm(t('confirmRevoke',r.name)))return;try{await api('relays/delete',{name:r.name});refresh()}catch(e){toast(e.message)}}));rels.append(it)}}

/* ---------- схема каскада ---------- */
const NS='http://www.w3.org/2000/svg';
function qual(r){return r<0?'bad':(r<100?'good':(r<200?'mid':'bad'))}
const QCOL={good:'var(--ok)',mid:'var(--ink2)',bad:'var(--accInk)'};
function srvAddr(){const s=state.server;return s.domain||s.public_ipv4||(s.public_ipv6?'['+s.public_ipv6+']':'—')}
function upInfo(name){return state.upstreams.find(u=>u.name===name)}
function upAddr(u){return u?(u.domain||u.ipv6||u.ipv4||'—'):'—'}
/* route — имена следующих узлов (без этого сервера). Задержки известны только от этого сервера до его соседних узлов. */
function buildGraph(route,editable,sel,canvas,size,avail,vertical){
const lat=latMap(),s=state.server;
const NW=editable?224:204,NH=editable?178:132,gx=100,gy=78;
const seq=[{k:'client'},{k:'this'}].concat(route.map(n=>({k:'up',name:n})),[{k:'net'}]);const last=seq.length-1;
const pos=seq.map((n,i)=>vertical?{x:16,y:16+i*(NH+gy)}:{x:20+i*(NW+gx),y:44});
const W=vertical?NW+32:40+seq.length*NW+(seq.length-1)*gx,Hh=vertical?32+seq.length*NH+(seq.length-1)*gy:NH+88;
canvas.replaceChildren();canvas.style.width=W+'px';canvas.style.height=Hh+'px';
const svg=document.createElementNS(NS,'svg');svg.setAttribute('width',W);svg.setAttribute('height',Hh);svg.setAttribute('aria-hidden','true');canvas.append(svg);
const rttOf=n=>{const x=lat[n.name];return x?(x.ok?x.rtt_ms:-1):null};
seq.forEach((n,idx)=>{const p=pos[idx];let title,rows,dotc='var(--ok)',key=n.k==='up'?n.name:n.k;
const role=idx===0?t('source'):(n.k==='net'?t('internet'):(idx===1&&idx===last-1?t('entryExit'):(idx===1?t('entry'):(idx===last-1?t('exit'):t('inter')))));
if(n.k==='client'){title=t('clients');rows=[[t('stClients'),s.clients],[t('stOnline'),s.online]];}
else if(n.k==='net'){title=t('internet');rows=[[t('addresses'),(s.public_ipv4&&s.public_ipv6)?'IPv4 + IPv6':(s.public_ipv6?'IPv6':'IPv4')],[t('via'),last>=2?(seq[last-1].k==='this'?t('thisServer'):seq[last-1].name):'—']]}
else if(n.k==='this'){title=t('thisServer');rows=[[t('address'),srvAddr()],[t('transport'),'HTTP/3 → HTTP/2'],[t('stClients'),s.clients]]}
else{const u=upInfo(n.name),r=rttOf(n);title=n.name;if(r!==null&&r<0)dotc='var(--accInk)';else if(r===null)dotc='var(--ink2)';
rows=[[t('address'),upAddr(u)],[r!==null&&r<0?t('status'):t('latency'),r===null?'—':(r<0?t('noLink'):ms1(r))],[t('stClients'),u?u.clients:'—']]}
const nd=el('div','gnode'+(sel&&sel===key?' sel':''));nd.style.cssText='left:'+p.x+'px;top:'+p.y+'px;width:'+NW+'px;height:'+NH+'px;animation-delay:'+(idx*0.05)+'s';
if(n.k!=='client'){const pi=el('span','pin in');pi.style.cssText=vertical?'left:'+(NW/2-6)+'px;top:-7px':'left:-8px;top:14px';nd.append(pi)}
if(n.k!=='net'){const po=el('span','pin out');po.style.cssText=vertical?'left:'+(NW/2-6)+'px;bottom:-7px':'right:-8px;top:14px';nd.append(po)}
const hd=el('button','ghead');hd.type='button';hd.setAttribute('aria-label',title);const dt=el('span','dot');dt.style.background=dotc;hd.append(dt,el('strong','',title),el('span','role'+(idx===1||idx===last-1?' key':''),role));
if(editable&&n.k!=='client'&&n.k!=='net')hd.onclick=()=>{bsel=key;renderBuilder()};nd.append(hd);
const gr=el('div','grows');rows.forEach(r=>{const d=el('div');d.append(el('span','',r[0]),el('span','',String(r[1])));gr.append(d)});nd.append(gr);
if(editable&&n.k==='up'){const ri=idx-2;const c=el('div','gctl');const mv=(d)=>{const j=ri+d;if(j<0||j>=bchain.length)return;const a=bchain[ri];bchain[ri]=bchain[j];bchain[j]=a;renderBuilder()};
const b1=btn(vertical?'↑':'←','',()=>mv(-1)),b2=btn(vertical?'↓':'→','',()=>mv(1)),b3=btn('✕','bad',()=>{bchain.splice(ri,1);if(bsel===key)bsel='this';renderBuilder()});
b1.setAttribute('aria-label',t('moveEarlier'));b2.setAttribute('aria-label',t('moveLater'));b3.setAttribute('aria-label',t('rmRoute'));c.append(b1,b2,b3);nd.append(c)}
canvas.append(nd);
if(idx<last){let q,label,anim=true;
if(idx===0){q=s.online>0?'good':'mid';label='—'}
else if(n.k==='this'&&seq[idx+1].k==='net'){q='good';label=t('exit')}
else if(seq[idx+1].k==='net'){q='good';label=t('exit')}
else{const r=idx===1?rttOf(seq[idx+1]):null;q=r===null?'mid':qual(r);label=r===null?'—':ms1(r)}
const q2=pos[idx+1],a=vertical?{x:p.x+NW/2,y:p.y+NH}:{x:p.x+NW,y:p.y+20},b=vertical?{x:q2.x+NW/2,y:q2.y}:{x:q2.x,y:q2.y+20};
const d=vertical?'M'+a.x+' '+a.y+' C'+a.x+' '+(a.y+gy*0.6)+','+b.x+' '+(b.y-gy*0.6)+','+b.x+' '+b.y:'M'+a.x+' '+a.y+' C'+(a.x+gx*0.6)+' '+a.y+','+(b.x-gx*0.6)+' '+b.y+','+b.x+' '+b.y;
const path=document.createElementNS(NS,'path');path.setAttribute('d',d);path.setAttribute('fill','none');path.setAttribute('stroke',QCOL[q]);path.setAttribute('stroke-width','2.5');path.setAttribute('stroke-dasharray',q!=='bad'?'3 9':'5 6');path.setAttribute('stroke-linecap','round');if(q!=='bad')path.style.animation='aFlow 1s linear infinite';svg.append(path);
const lb=el('span','glabel');lb.style.cssText='left:'+((a.x+b.x)/2-38)+'px;top:'+((a.y+b.y)/2-13)+'px';const bb=el('b','',label);lb.append(bb);canvas.append(lb)}});
const sc=(editable||vertical)?1:Math.max(0.72,Math.min(1,avail/W));
canvas.style.transform='scale('+sc+')';size.style.width=Math.round(W*sc)+'px';size.style.height=Math.round(Hh*sc)+'px'}

function pickClient(){const c=state.clients.filter(c=>!c.relay_of&&c.route&&c.route.length);c.sort((a,b)=>((b.online?1:0)-(a.online?1:0))||(new Date(b.last_handshake||0)-new Date(a.last_handshake||0)));return c[0]||null}
function renderScheme(){const c=pickClient(),route=c?c.route:[];const lat=latMap();
const sum=$('g-sum');sum.replaceChildren();
const chip=(l,v,acc)=>{const e=el('span','tag'+(acc?' acc':''));e.append(l+' ');e.append(el('b','',v));sum.append(e)};
chip(t('nodesN'),String(route.length+1));
if(c){chip(t('viaClient'),c.name);chip(t('mode'),c.auto?t('modeAuto'):t('modeManual'));
const on=route.filter(n=>{const x=lat[n];return x&&x.ok}).length;chip(t('onlineL'),on+'/'+route.length);
const f=lat[route[0]];if(f&&f.ok)chip(t('firstHop'),ms1(f.rtt_ms));
if(c.auto&&c.auto.cost_ms>0)chip(t('total'),ms1(c.auto.cost_ms),true)}
else{const e=el('span','tag',t('schemeNone'));sum.append(e)}
const wrap=$('g-wrap');const vertical=window.innerWidth<640;
buildGraph(route,false,null,$("g-canvas"),$("g-size"),wrap.clientWidth,vertical);
const lg=$('g-legend');lg.replaceChildren();
[['3px dotted var(--ok)',t('lgOk')],['3px dotted var(--ink2)',t('lgSlow')],['3px dashed var(--accInk)',t('lgBad')]].forEach(x=>{const s=el('span');const i=el('span','lg');i.style.borderTop=x[0];s.append(i,x[1]);lg.append(s)});
const s4=el('span');s4.append(el('span','lg'),t('lgUnk'));s4.firstChild.style.cssText='width:10px;height:10px;border-radius:3px;background:var(--acc)';lg.append(s4)}

/* ---------- конструктор ---------- */
let bchain=[],bmode='manual',bsel='this';
function bOrder(){let r=bchain.slice();if(bmode==='auto'&&r.length>2){const lat=latMap();const k=n=>{const x=lat[n];return x&&x.ok?x.rtt_ms:1e9};r=[r[0]].concat(r.slice(1,-1).sort((a,b)=>k(a)-k(b)),[r[r.length-1]])}return r}
function openBuilder(){const c=pickClient();bchain=c?c.route.filter(n=>upInfo(n)):[];bmode=c&&c.auto?'auto':'manual';bsel='this';$('builder').classList.add('show');renderBuilder()}
function renderBuilder(){const lat=latMap(),s=state.server;const ord=bOrder();const key=bsel==='this'||ord.includes(bsel)?bsel:'this';bsel=key;
const m=$('b-modes');m.replaceChildren();[['manual',t('mManual')],['auto',t('mAuto')]].forEach(x=>{const b=btn(x[1],bmode===x[0]?'on':'',()=>{bmode=x[0];renderBuilder()});m.append(b)});
const pal=$('b-pal');pal.replaceChildren();
state.upstreams.filter(u=>!bchain.includes(u.name)).forEach(u=>{const x=lat[u.name];const bad=!x||!x.ok;const d=el('div','pitem'+(bad?' off':''));const l=el('div');
l.append(el('strong','',u.name),el('div','',upAddr(u)+' · '+(x?(x.ok?ms1(x.rtt_ms):t('noLink')):t('measuring'))));
const b=btn('+','',()=>{if(bad)return;if(bchain.length>=1)bchain.splice(bchain.length-1,0,u.name);else bchain.push(u.name);renderBuilder()});b.setAttribute('aria-label',t('addRoute',u.name));d.append(l,b);pal.append(d)});
buildGraph(ord,true,key,$('b-canvas'),$('b-size'),0,window.innerWidth<640);
const wr=$('b-warns');wr.replaceChildren();const w=(cls,x)=>wr.append(el('div',cls,x));
const offs=ord.filter(n=>{const x=lat[n];return !x||!x.ok});
if(ord.length+1>5)w('bad',t('wMany'));if(offs.length)w('bad',t('wOff',offs.join(', ')));if(bmode==='auto')w('info',t('wAuto'));if(!wr.children.length)w('ok',t('wOk'));
/* инспектор */
const idx=key==='this'?0:ord.indexOf(key)+1,n=ord.length+1,isUp=key!=='this';const u=isUp?upInfo(key):null,x=isUp?lat[key]:null;
$('i-title').textContent=isUp?key:t('thisServer');
const rr=[[t('role'),n===1?t('entryExit'):(idx===0?t('entry'):(idx===n-1?t('exit'):t('inter')))],[t('address'),isUp?upAddr(u):srvAddr()]];
if(isUp){if(u&&u.company)rr.push([t('company'),u.company]);rr.push([t('latFromThis'),x?(x.ok?ms1(x.rtt_ms):'—'):'—'])}else rr.push([t('transport'),'HTTP/3 → HTTP/2']);
rr.push([t('stClients'),isUp?(u?String(u.clients):'—'):String(s.clients)],[t('position'),(idx+1)+' / '+n]);
const ir=$('i-rows');ir.replaceChildren();rr.forEach(r=>{const d=el('div','irow');d.append(el('span','',r[0]),el('b','',r[1]));ir.append(d)});
$('i-bars').parentNode.style.display=isUp?'block':'none';const bars=$('i-bars');bars.replaceChildren();const hs=isUp?(hist[key]||[]):[];$('i-histcap').textContent=isUp?t('histCap'):'';
if(isUp&&!hs.length)bars.append(el('span','muted',t('noHistory')));
const mx=Math.max.apply(null,hs.concat([1]));hs.forEach((v,i)=>{const b=el('div',v<0?'fail':(i===hs.length-1?'last':''));b.style.height=(v<0?8:Math.max(8,Math.round(100*v/mx)))+'%';bars.append(b)});
$('i-route').textContent=[t('clients'),t('thisServer')].concat(ord,[t('internet')]).join(' → ');
const f=ord.length?lat[ord[0]]:null;$('i-total').textContent=(f&&f.ok?t('routeTotalFirst',ms1(f.rtt_ms))+' · ':t('routeTotalUnk')+' · ')+n+' '+t('nodesShort')}
$('b-byKey').onclick=()=>$('up-add').click();$('b-issueKey').onclick=()=>$('rel-add').click();
$('open-builder').onclick=openBuilder;$('b-close').onclick=()=>$('builder').classList.remove('show');
$('b-check').onclick=async()=>{const ord=bOrder();try{await api('explore',{route:ord});toast(t('checkOk',ord.length+1))}catch(e){toast(e.message,4000)}};
$('b-apply').onclick=()=>{const ord=bOrder();if(!ord.length){toast(t('applyEmpty'));return}$('builder').classList.remove('show');openNewClient(bmode==='auto'&&ord.length>1?{mode:'auto',exit:ord[ord.length-1],hops:ord.length-1}:{mode:'manual',route:ord})};

/* ---------- диалоги ---------- */
let route=[],editRole='user';
async function renderRoute(){const box=$('c-route');box.replaceChildren();
route.forEach((h,i)=>{const d=el('div','hop');d.append(el('span','tag',(i+1)+'. '+h));if(i===route.length-1)d.append(btn(t('undo'),'lnk',()=>{route.pop();renderRoute()}));box.append(d)});
let opts=[];try{opts=(await api('explore',{route})).upstreams||[]}catch(e){box.append(el('div','err',e.message));return}
opts=opts.filter(o=>!route.includes(o));const d=el('div','hop');const w=el('div','sel flat');const sel=el('select');sel.append(new Option(route.length?t('exitOnLast'):t('exitOnThis'),''));for(const o of opts)sel.append(new Option(t('nextThrough',o),o));
sel.onchange=()=>{if(sel.value){route.push(sel.value);renderRoute()}};w.append(sel);if(opts.length)d.append(w);else d.append(el('span','muted',route.length?t('noMoreNodes'):t('noCascade')));box.append(d)}
function syncMode(){const auto=$('c-mode').value==='auto';$('c-auto').style.display=auto?'block':'none';$('c-manual').style.display=auto?'none':'block'}
$('c-mode').onchange=syncMode;
function openNewClient(pre){pre=pre||{};$('c-name').value='';$('c-note').value='';$('c-role').value='user';$('c-warn').style.display='none';$('c-err').textContent='';route=(pre.route||[]).slice();renderRoute();
const ex=$('c-exit');ex.replaceChildren();for(const u of state.upstreams)ex.append(new Option(u.name+' ('+u.company+')',u.name));
const hp=$('c-hops');hp.replaceChildren();hp.append(new Option(t('allAvail'),'-1'));for(let i=0;i<state.upstreams.length;i++)hp.append(new Option(String(i),String(i)));
if(pre.mode==='auto'){ex.value=pre.exit;hp.value=String(pre.hops)}
$('c-mode').value=pre.mode==='auto'?'auto':'manual';$('c-mode').disabled=!state.upstreams.length;syncMode();open('m-client');$('c-name').focus()}
$('add').onclick=()=>openNewClient();
$('optimize').onclick=async()=>{$('optimize').disabled=true;try{const r=await api('cascade/optimize',{});const ev=r.events||[];toast(ev.length?ev.map(e=>e.error?(e.client+': '+e.error):(e.client+': '+e.from+' → '+e.to+' ('+(e.old_ms<0?t('unreachable'):ms1(e.old_ms))+' → '+ms1(e.new_ms)+')')).join('\n'):t('optNone'),ev.length?6000:4000);refresh()}catch(e){toast(e.message)}finally{$('optimize').disabled=false}};
$('c-create').onclick=async()=>{$('c-create').disabled=true;try{const auto=$('c-mode').value==='auto';const body=auto?{name:$('c-name').value,note:$('c-note').value,role:$('c-role').value,auto:true,exit:$('c-exit').value,max_hops:parseInt($('c-hops').value,10)}:{name:$('c-name').value,note:$('c-note').value,route,role:$('c-role').value};const r=await api('clients',body);close('m-client');showLink(t('createdTitle',r.name),r,t('createdWarn'));refresh()}catch(e){$('c-err').textContent=e.message}finally{$('c-create').disabled=false}};
$('e-save').onclick=async()=>{try{const body={name:editing,new_name:$('e-name').value,note:$('e-note').value};if($('e-role').value!==editRole)body.role=$('e-role').value;await api('clients/update',body);close('m-edit');refresh()}catch(e){$('e-err').textContent=e.message}};
$('ro-save').onclick=async()=>{try{await api('clients/update',{name:editing,role:$('ro-role').value});close('m-role');refresh()}catch(e){$('ro-err').textContent=e.message}};
$('c-role').onchange=()=>{$('c-warn').style.display=$('c-role').value==='admin'?'block':'none'};
$('r-admin').onchange=()=>{$('r-warn').style.display=$('r-admin').value==='1'?'block':'none'};
$('up-add').onclick=()=>{$('u-name').value='';$('u-link').value='';$('u-err').textContent='';open('m-up')};
$('u-save').onclick=async()=>{$('u-save').disabled=true;try{await api('upstreams',{name:$('u-name').value,link:$('u-link').value});close('m-up');refresh()}catch(e){$('u-err').textContent=e.message}finally{$('u-save').disabled=false}};
$('rel-add').onclick=()=>{$('r-name').value='';$('r-admin').value='0';$('r-warn').style.display='none';$('r-err').textContent='';open('m-rel')};
$('r-save').onclick=async()=>{try{const r=await api('relays',{name:$('r-name').value,admin:$('r-admin').value==='1'});close('m-rel');showLink(t('relKeyTitle',r.name),r,t('relKeyWarn',r.name));refresh()}catch(e){$('r-err').textContent=e.message}};
const FRONT_PRESETS=['ya.ru','yandex.ru','vk.com','mail.ru'];
function frontChoice(){const v=$('front-sel').value;return v==='custom'?$('front-domain').value.trim().toLowerCase():v}
function renderFront(s){const f=s.front||{};const sel=$('front-sel');
if(document.activeElement!==sel&&document.activeElement!==$('front-domain')){
sel.replaceChildren(new Option(t('frontNone'),''));for(const d of FRONT_PRESETS)sel.append(new Option(d,d));sel.append(new Option(t('frontOther'),'custom'));
if(!f.enabled)sel.value='';else if(FRONT_PRESETS.includes(f.sni))sel.value=f.sni;else{sel.value='custom';$('front-domain').value=f.sni}
$('front-custom').style.display=sel.value==='custom'?'flex':'none'}
$('front-state').textContent=f.enabled?t('frontOn',f.sni):t('frontOff');
$('front-note').textContent=f.enabled&&s.domain?t('frontDomainNote'):''}
$('front-sel').onchange=()=>{$('front-custom').style.display=$('front-sel').value==='custom'?'flex':'none'};
$('front-save').onclick=async()=>{const sni=frontChoice();const cur=state.server.front||{};
if($('front-sel').value==='custom'&&!sni){toast(t('frontBad'));return}
if(sni&&sni!==cur.sni&&!confirm(t('frontConfirm',sni)))return;
try{await api('settings/front',{sni,dest:sni&&sni===cur.sni?cur.dest:''});toast(t(sni?'frontDoneOn':'frontDoneOff'));refresh()}catch(e){toast(e.message)}};
let srDirty=false;
['sr-url','sr-ref','sr-dir','sr-token'].forEach(id=>$(id).addEventListener('input',()=>{srDirty=true}));
function renderSite(){const r=state.site_repo||{};const st=$('sr-status'),bar=$('sr-bar'),er=$('sr-err');
let txt,cls='tag';
if(r.syncing){txt=t('srBusy');cls='tag acc'}
else if(r.error){txt=t('srErr');cls='tag bad'}
else if(!r.enabled){txt=t('srNone')}
else if(r.active&&r.updated){txt=t('srOk',ago(r.updated))}
else txt=t('srWait');
st.className=cls;st.textContent=txt;
bar.style.display=r.syncing?'block':'none';
const e=r.error&&!r.syncing;er.style.display=e?'block':'none';if(e)er.textContent=t('srErrText',r.error);
const mt=$('sr-meta');const m=r.enabled&&r.active&&r.commit;mt.style.display=m?'inline':'none';if(m)mt.textContent=t('srMeta',String(r.commit).slice(0,8),r.files||0);
if(!srDirty&&!['sr-url','sr-ref','sr-dir','sr-token'].some(id=>document.activeElement===$(id))){$('sr-url').value=r.url||'';$('sr-ref').value=r.ref||'';$('sr-dir').value=r.dir||'';$('sr-token').value='';$('sr-token').placeholder=r.has_token?'••••••••••••':''}
$('sr-update').disabled=!!r.syncing;$('sr-reset').disabled=!r.enabled;$('sr-clrtok').style.display=r.enabled&&r.has_token?'inline-block':'none'}
$('sr-update').onclick=async()=>{const r=state.site_repo||{};const url=$('sr-url').value.trim();if(!url){toast(t('srNeedUrl'));return}
const same=r.enabled&&url===r.url&&$('sr-ref').value.trim()===(r.ref||'')&&$('sr-dir').value.trim()===(r.dir||'')&&!$('sr-token').value;
$('sr-update').disabled=true;try{if(same)await api('settings/siterepo/refresh',{});else await api('settings/siterepo',{url,ref:$('sr-ref').value.trim(),dir:$('sr-dir').value.trim(),token:$('sr-token').value});srDirty=false;$('sr-token').value='';toast(t('srQueued'));await refresh()}catch(e){toast(e.message,4000)}finally{$('sr-update').disabled=false}};
$('sr-clrtok').onclick=async()=>{const r=state.site_repo||{};if(!r.has_token||!confirm(t('srClrConfirm')))return;try{await api('settings/siterepo',{url:r.url,ref:r.ref||'',dir:r.dir||'',clear_token:true});srDirty=false;toast(t('srClrDone'));await refresh()}catch(e){toast(e.message,4000)}};
$('sr-reset').onclick=async()=>{try{await api('settings/siterepo',{url:''});srDirty=false;toast(t('srResetDone'));await refresh()}catch(e){toast(e.message)}};
$('domain-save').onclick=async()=>{try{await api('settings/domain',{domain:$('domain').value});refresh()}catch(e){toast(e.message)}};
$('pw-save').onclick=async()=>{$('pw-err').textContent='';if($('pw-new').value!==$('pw-rep').value){$('pw-err').textContent=t('pwMismatch');return}
try{await api('settings/password',{current:$('pw-cur').value,new:$('pw-new').value});toast(t('pwDone'),1500);setTimeout(()=>{location.href='/admin'},1500)}catch(e){$('pw-err').textContent=e.message}};
$('l-copy').onclick=async()=>{try{await navigator.clipboard.writeText($('l-text').value);$('l-copy').textContent=t('copied')}catch(e){$('l-text').select()}setTimeout(()=>$('l-copy').textContent=t('copy'),1500)};
$('l-dl').onclick=()=>{const a=document.createElement('a');a.href=URL.createObjectURL(new Blob([$('l-text').value+'\n'],{type:'application/octet-stream'}));a.download=dlName;a.click();URL.revokeObjectURL(a.href)};
$('logout').onclick=async()=>{try{await api('../logout',{})}catch(e){}location.href='/admin'};
$('lang-btn').onclick=()=>{lang=lang==='ru'?'en':'ru';store('veil-lang',lang);applyStatic();if(state)render()};
$('theme-btn').onclick=()=>{theme=theme==='dark'?'light':'dark';store('veil-theme',theme);applyStatic()};
let rz=0;window.addEventListener('resize',()=>{clearTimeout(rz);rz=setTimeout(()=>{if(state){renderScheme();if($('builder').classList.contains('show'))renderBuilder()}},120)});
applyStatic();refresh().then(()=>setTimeout(()=>document.body.classList.remove("fresh"),1200));setInterval(()=>{if(!document.querySelector('.modal.show'))refresh()},3000);
(async()=>{try{const r=await api('cascade/nodes');nodes=r.nodes||[];renderTargets()}catch(e){}})();
</script>
</body></html>`
