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

// panelHTML — одностраничная админ-панель (/admin): только встроенные стили и скрипт (CSP), данные — через /admin/api/….
const panelHTML = `<!doctype html>
<html lang="ru" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Veil Admin</title>
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
:root,:root[data-theme=dark]{--bg:#0B0B0C;--card:#1B1C1F;--card2:#2A2B30;--card3:#3A3C42;--line:#2A2B30;--ink:#FFFFFF;--ink2:#A8AAB2;--ink3:#6B6F78;--hi:#D9DBE0;--acc:#FF393D;--onacc:#0B0B0C;--accInk:#FF393D;--ok:#7DE0A0;--warn:#FFB020;--blue:#9DB8FF;--dngBg:#2A1416;--dngInk:#FF8A8C;--tx:#6B6F78;--off:#4A4D55;--selrow:rgba(34,35,39,.5);--scrim:rgba(11,11,12,.6);--scrim2:rgba(11,11,12,.78);--shadow:0 4px 12px rgba(0,0,0,.18);color-scheme:dark}
:root[data-theme=light]{--bg:#FFFFFF;--card:#F2F3F5;--card2:#E4E6EA;--card3:#D5D8DD;--line:#E4E6EA;--ink:#0B0B0C;--ink2:#5B6068;--ink3:#8A8F98;--hi:#2A2B30;--acc:#FF393D;--onacc:#0B0B0C;--accInk:#DB1E32;--ok:#177A44;--warn:#A86400;--blue:#3555B0;--dngBg:#FDE8E9;--dngInk:#B3151F;--tx:#A8AAB2;--off:#C3C6CC;--selrow:rgba(0,0,0,.035);--scrim:rgba(11,11,12,.35);--scrim2:rgba(11,11,12,.5);--shadow:0 4px 12px rgba(0,0,0,.12);color-scheme:light}
*{box-sizing:border-box}
html,body{margin:0}
body{background:var(--bg);color:var(--ink);font-family:Montserrat,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;font-size:14px;line-height:1.4;-webkit-text-size-adjust:100%}
button,input,textarea{font-family:inherit}
button:focus-visible,a:focus-visible,input:focus-visible,textarea:focus-visible{outline:2px solid var(--accInk);outline-offset:2px}
input::placeholder,textarea::placeholder{color:var(--ink3)}
*{scrollbar-width:thin;scrollbar-color:var(--card2) transparent}
::-webkit-scrollbar{width:10px;height:10px;background:transparent}
::-webkit-scrollbar-thumb{background:var(--card2);border-radius:5px;border:3px solid transparent;background-clip:padding-box}
@keyframes spin{to{transform:rotate(360deg)}}
@keyframes aFlow{to{stroke-dashoffset:-12}}
@keyframes aFade{from{opacity:0}to{opacity:1}}
@keyframes aRise{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
@keyframes aSlide{from{transform:translateX(40px);opacity:.4}to{transform:none;opacity:1}}
@keyframes aInd{from{margin-left:-40%}to{margin-left:100%}}
@media (prefers-reduced-motion:reduce){*{animation-duration:.001s!important;transition-duration:.001s!important}}
.num,code,.mono{font-family:Arimo,Arial,sans-serif}
.muted{color:var(--ink2)}
.grow{flex:1;min-width:0}
.ell{white-space:nowrap;overflow:hidden;text-overflow:ellipsis;min-width:0}
.sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)}
.ico{display:inline-flex;align-items:center;justify-content:center;flex:none}
.dot{display:inline-block;width:8px;height:8px;border-radius:50%;flex:none;background:var(--ink2)}

.app{display:flex;height:100vh;height:100dvh}
.side{width:236px;flex:none;background:var(--card);border-right:1px solid var(--line);display:flex;flex-direction:column;min-height:0}
.brand{padding:16px 16px 10px;display:flex;align-items:center;gap:10px}
.brand .bt{display:flex;flex-direction:column;min-width:0}
.brand b{font-size:15px;font-weight:700}
.brand small{font-size:11px;font-weight:500;color:var(--ink2)}
.mbtn{display:none!important;margin-left:auto}
.nodesw{position:relative;padding:0 12px 10px}
.nodebtn{width:100%;height:48px;border:0;border-radius:11px;background:var(--card2);color:var(--ink);display:flex;align-items:center;gap:10px;padding:0 12px;text-align:left;cursor:pointer}
.nodebtn:disabled{cursor:default}
.nodebtn .ico{color:var(--ink2)}
.lbl2{display:flex;flex-direction:column;flex:1;min-width:0}
.lbl2 small{font-size:11px;font-weight:500;color:var(--ink2);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.lbl2 span{font-size:13px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.pop{position:absolute;left:12px;right:12px;top:52px;z-index:20;border-radius:12px;background:var(--card2);box-shadow:var(--shadow);padding:6px;display:flex;flex-direction:column;gap:2px;max-height:320px;overflow:auto}
.opt{width:100%;min-height:44px;border:0;border-radius:9px;background:transparent;color:var(--ink);display:flex;align-items:center;gap:10px;padding:6px 10px;text-align:left;cursor:pointer}
.opt.on,.opt:hover{background:var(--card3)}
nav.nav{flex:1;overflow:auto;padding:4px 12px;display:flex;flex-direction:column;gap:2px}
.ngrp{font-size:11px;font-weight:600;color:var(--ink3);padding:12px 10px 4px}
.nitem{height:38px;flex:none;border:0;border-radius:9px;background:transparent;color:var(--ink2);display:flex;align-items:center;gap:10px;padding:0 10px;font-size:13px;font-weight:600;cursor:pointer;text-align:left}
.nitem:hover{color:var(--ink)}
.nitem.cur{background:var(--card2);color:var(--ink)}
.badge{min-width:20px;height:20px;border-radius:9999px;background:var(--card2);color:var(--hi);font:700 11px Arimo,Arial,sans-serif;display:flex;align-items:center;justify-content:center;padding:0 6px}
.nitem.cur .badge{background:var(--card3)}
.badge.warn,.nitem.cur .badge.warn{background:var(--acc);color:var(--onacc)}
.sfoot{border-top:1px solid var(--line);padding:12px 16px;display:flex;align-items:center;gap:10px}
.ava{width:32px;height:32px;border-radius:50%;background:var(--card2);display:flex;align-items:center;justify-content:center;font-size:13px;font-weight:700;flex:none}
.ib{width:32px;height:32px;flex:none;border:0;border-radius:9px;background:transparent;color:var(--ink2);cursor:pointer;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:700}
.ib:hover{background:var(--card2);color:var(--ink)}
.ib.lg{width:36px;height:36px;background:var(--card)}

.mainc{flex:1;min-width:0;display:flex;flex-direction:column}
header.top{height:64px;flex:none;border-bottom:1px solid var(--line);display:flex;align-items:center;gap:14px;padding:0 24px}
.crumb{display:flex;flex-direction:column;min-width:0}
.crumb small{font-size:11px;font-weight:500;color:var(--ink2);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
h1{margin:0;font-size:20px;line-height:26px;font-weight:600}
.search{width:300px;height:36px;border-radius:9px;background:var(--card);display:flex;align-items:center;gap:8px;padding:0 10px;color:var(--ink2);position:relative}
.search:focus-within{outline:2px solid var(--accInk);outline-offset:2px}
.search input{flex:1;border:0;background:transparent;color:var(--ink);font:500 13px Montserrat,system-ui,sans-serif;min-width:0;outline:0;height:100%}
.health{height:32px;border-radius:9px;background:var(--card);display:flex;align-items:center;gap:8px;padding:0 12px;font-size:12px;font-weight:600;border:0;cursor:pointer;white-space:nowrap}
.hdbtns{display:flex;gap:6px}
.notice{margin:16px 24px 0;display:flex;align-items:center;gap:12px;flex-wrap:wrap}
.notice>div{flex:1;min-width:200px}
.notice b{display:block;font-size:13px}
main{flex:1;min-height:0;overflow:auto;padding:20px 24px 24px;display:flex;flex-direction:column;gap:16px}
main.dim{opacity:.45;pointer-events:none}

.b{height:32px;border:0;border-radius:9px;padding:0 12px;background:var(--card2);color:var(--ink);font-size:12px;font-weight:600;cursor:pointer;white-space:nowrap;display:inline-flex;align-items:center;justify-content:center;gap:8px;flex:none}
.b.lg{height:36px;padding:0 14px;font-size:13px}
.b.xl{height:40px;padding:0 16px;font-size:13px}
.b.sm{height:30px;border-radius:8px}
.b.acc{background:var(--acc);color:var(--onacc)}
.b.wht{background:var(--ink);color:var(--bg)}
.b.red{color:var(--accInk)}
.b.ghost{background:transparent}
.b.flat{background:var(--card)}
.b:disabled{opacity:.5;cursor:default}
.lnkb{border:0;background:transparent;color:var(--accInk);font-size:12px;font-weight:600;cursor:pointer;padding:0;white-space:nowrap}
.card{border-radius:14px;background:var(--card);padding:16px 18px;min-width:0;display:flex;flex-direction:column;gap:10px}
.card.p0{padding:0;overflow:hidden;gap:0}
.card h2{margin:0;font-size:15px;font-weight:600}
.chd{padding:14px 16px;display:flex;align-items:center;gap:10px;flex-wrap:wrap}
.chd h2{flex:1;min-width:0}
.g2{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:16px;align-items:start}
.g21{display:grid;grid-template-columns:minmax(0,2fr) minmax(0,1fr);gap:16px;align-items:start}
.g141{display:grid;grid-template-columns:minmax(0,1.4fr) minmax(0,1fr);gap:16px;align-items:start}
.gcv{display:grid;grid-template-columns:minmax(0,1fr) 420px;gap:16px;align-items:start}
.g3{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}
.g5{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px}
.row{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.col{display:flex;flex-direction:column;gap:8px}
.kpi{border:0;border-radius:14px;background:var(--card);color:var(--ink);padding:14px 16px;display:flex;flex-direction:column;gap:6px;text-align:left;cursor:pointer;min-width:0}
.kpi>span{font-size:12px;font-weight:500;color:var(--ink2)}
.kpi b{font:700 26px Arimo,Arial,sans-serif;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.kpi small{font-size:12px;font-weight:600;color:var(--ink2);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
div.kpi{cursor:default}
.note{margin:0;font-size:12px;line-height:17px;font-weight:500;color:var(--ink2)}
.note.big{font-size:13px;line-height:19px}
.box{border-radius:11px;background:var(--bg);padding:10px 12px;font-size:12px;line-height:17px;font-weight:500;color:var(--ink2)}
main>.box{background:var(--card)}
.dng{border-radius:11px;background:var(--dngBg);color:var(--dngInk);padding:10px 12px;font-size:12px;line-height:17px;font-weight:600}
.chip{height:32px;border:0;border-radius:9999px;padding:0 14px;background:var(--card);color:var(--ink);font-size:12px;font-weight:600;cursor:pointer;white-space:nowrap}
.chip.on{background:var(--ink);color:var(--bg)}
.chip.in{background:var(--card2)}
.chip.in.on{background:var(--acc);color:var(--onacc)}
.tw{overflow-x:auto}
table.t{width:100%;border-collapse:collapse;font-size:13px}
.t th{height:40px;color:var(--ink2);text-align:left;font-size:12px;font-weight:600;padding:0 8px;white-space:nowrap}
.t td{padding:6px 8px;height:52px;border-top:1px solid var(--line)}
.t.c44 td{height:44px}.t.c48 td{height:48px}
.t th:first-child,.t td:first-child{padding-left:16px}
.t th:last-child,.t td:last-child{padding-right:16px}
.t .r{text-align:right}
.t td.sm{font:400 12px Arimo,Arial,sans-serif;color:var(--ink2)}
.t td.addr{font:400 12px Arimo,Arial,sans-serif;color:var(--hi)}
.t tr.sel td{background:var(--selrow)}
.cbtn{border:0;background:transparent;color:var(--ink);padding:0;text-align:left;cursor:pointer;display:flex;flex-direction:column;gap:1px;max-width:260px}
.cbtn b{font-size:13px;font-weight:600;display:flex;gap:6px;align-items:center}
.cbtn small{font-size:11px;font-weight:500;color:var(--ink2)}
.gw{font-size:10px;font-weight:700;color:var(--bg);background:var(--hi);border-radius:5px;padding:1px 5px}
.cb{width:20px;height:20px;border-radius:6px;border:2px solid var(--ink3);background:transparent;padding:0;display:flex;align-items:center;justify-content:center;cursor:pointer;color:var(--onacc)}
.cb.on{background:var(--acc);border-color:var(--acc)}
.cb:disabled{opacity:.3;cursor:default}
.cb i{width:10px;height:3px;border-radius:2px;background:var(--onacc)}
.pill{display:inline-block;font-size:11px;font-weight:700;border-radius:6px;padding:3px 7px;white-space:nowrap}
.st{display:flex;align-items:center;gap:6px;font-size:12px;font-weight:600;white-space:nowrap}
.st .dot{width:7px;height:7px}
.tfoot{height:44px;border-top:1px solid var(--line);display:flex;align-items:center;gap:10px;padding:0 16px;font-size:12px;font-weight:500;color:var(--ink2)}
.empty{padding:40px;text-align:center;font-size:13px;font-weight:500;color:var(--ink2);border-top:1px solid var(--line)}
.bulk{min-height:48px;border-radius:12px;background:var(--card2);display:flex;align-items:center;gap:10px;padding:8px 14px;flex-wrap:wrap}
.bulk>span{font-size:13px;font-weight:600;flex:1}
.bulk .b{background:var(--card)}
.sw{width:40px;height:24px;border-radius:9999px;background:var(--card2);position:relative;flex:none;display:inline-block}
.sw i{position:absolute;top:3px;left:3px;width:18px;height:18px;border-radius:50%;background:var(--ink2);transition:left .15s}
.sw.on{background:var(--acc)}
.sw.on i{left:19px;background:var(--onacc)}
.sw.s{width:28px;height:16px;background:var(--off)}
.sw.s i{top:2px;left:2px;width:12px;height:12px;background:var(--onacc)}
.sw.s.on{background:var(--acc)}
.sw.s.on i{left:14px}
.card .sw:not(.on),.modal .sw:not(.on),.drawer .sw:not(.on){background:var(--card3)}
.swrow{width:100%;border:0;border-radius:11px;background:var(--bg);color:var(--ink);padding:12px;display:flex;align-items:center;gap:10px;text-align:left;cursor:pointer}
.rad{width:100%;border:0;border-radius:12px;background:var(--bg);color:var(--ink);padding:12px 14px;display:flex;gap:12px;align-items:center;text-align:left;cursor:pointer}
.rad.on{box-shadow:inset 0 0 0 2px var(--acc)}
.rad .rc{width:20px;height:20px;border-radius:50%;border:2px solid var(--ink3);display:flex;align-items:center;justify-content:center;flex:none}
.rad .rc i{width:10px;height:10px;border-radius:50%}
.rad.on .rc{border-color:var(--acc)}
.rad.on .rc i{background:var(--acc)}
.tt{display:flex;flex-direction:column;gap:2px;min-width:0}
.tt b{font-size:13px;font-weight:600}
.rad .tt b{font-size:14px}
.drawer .rad .tt b,.modal .rad .tt b{font-size:13px}
.tt small{font-size:12px;line-height:17px;font-weight:500;color:var(--ink2)}
.fld{display:flex;flex-direction:column;gap:6px;font-size:12px;font-weight:600;color:var(--ink2);min-width:0}
.flab{font-size:12px;font-weight:600;color:var(--ink2)}
.inp{display:block;width:100%;height:38px;border:0;border-radius:9px;background:var(--card2);color:var(--ink);padding:0 10px;font:500 13px Montserrat,system-ui,sans-serif}
.inp.lg{height:40px;padding:0 12px;font-size:14px}
.inp.mono{font:400 13px Arimo,Arial,sans-serif}
textarea.inp{height:110px;padding:10px;font:400 13px/19px Arimo,Arial,sans-serif;resize:vertical}
.inp.bad{box-shadow:inset 0 0 0 2px var(--acc)}
.ddw{position:relative;display:flex;flex-direction:column;gap:6px}
.ddw.open{z-index:30}
.ddb{height:40px;border:0;border-radius:11px;background:var(--card2);color:var(--ink);display:flex;align-items:center;gap:10px;padding:0 12px;width:100%;text-align:left;cursor:pointer;font-size:13px;font-weight:600}
.ddb .ico{color:var(--ink2);transition:transform .15s}
.ddb.open{box-shadow:inset 0 0 0 2px var(--acc)}
.ddb.open .ico{transform:rotate(180deg)}
.ddb:disabled{opacity:.5;cursor:default}
.ddl{position:absolute;left:0;right:0;top:calc(100% + 4px);border-radius:12px;background:var(--card2);box-shadow:var(--shadow);padding:6px;display:flex;flex-direction:column;gap:2px;max-height:280px;overflow:auto}
.ddo{min-height:38px;border:0;border-radius:8px;background:transparent;color:var(--ink);display:flex;align-items:center;gap:10px;padding:4px 10px;cursor:pointer;font-size:13px;font-weight:600;text-align:left}
.ddo.on,.ddo:hover{background:var(--card3)}
.ddo .ico{color:var(--accInk)}
.kv{display:grid;grid-template-columns:auto minmax(0,1fr);gap:8px 16px;font-size:13px}
.kv>span:nth-child(odd){color:var(--ink2)}
.kv>span{min-width:0;overflow-wrap:anywhere}
.lrow{min-height:40px;display:flex;align-items:center;gap:10px;border-top:1px solid var(--line);font-size:13px}
.lrow .lname{font-weight:600;width:140px;flex:none}
.lrow .tm{font:400 12px Arimo,Arial,sans-serif;color:var(--ink2);width:44px;flex:none}
.lrow .kd{font-size:11px;font-weight:700;width:84px;flex:none}
.irow{min-height:52px;border-top:1px solid var(--line);display:flex;align-items:center;gap:10px;padding:6px 16px;font-size:13px}
.irow.h48{min-height:48px;gap:12px}
.sm{font-size:12px}
.alert{border:0;border-radius:11px;background:var(--bg);color:var(--ink);padding:10px 12px;display:flex;gap:10px;align-items:flex-start;text-align:left;cursor:pointer;width:100%}
.alert .dot{margin-top:5px}
.chart{height:190px;display:flex;align-items:flex-end;gap:5px;border-bottom:1px solid var(--line)}
.chart>div{flex:1;display:flex;flex-direction:column;justify-content:flex-end;gap:2px;height:100%;overflow:hidden;min-width:0}
.chart i{display:block;background:var(--acc);flex:none}
.chart i.tx{background:var(--tx)}
.axis{display:flex;justify-content:space-between;font:400 11px Arimo,Arial,sans-serif;color:var(--ink2)}
.lg{display:flex;align-items:center;gap:6px;font-size:12px;font-weight:500;color:var(--ink2)}
.lg i{width:10px;height:10px;background:var(--acc)}
.lg i.tx{background:var(--tx)}
.lcard{border:0;border-radius:14px;background:var(--card);color:var(--ink);padding:16px;display:flex;flex-direction:column;gap:8px;text-align:left;cursor:pointer;min-width:0}
.lcard.ch{box-shadow:inset 0 0 0 2px var(--acc)}
.lcard .lt{display:flex;align-items:center;gap:10px;width:100%;font-size:15px;font-weight:600}
.verdict{border-radius:11px;background:var(--bg);padding:12px;display:flex;gap:10px;align-items:center}
.verdict>.dot{width:10px;height:10px}
.chain{display:flex;align-items:center}
.cw{display:flex;align-items:center;flex:1;min-width:0}
.cn{flex:1;min-width:0;border-radius:12px;background:var(--bg);padding:12px;display:flex;flex-direction:column;gap:3px}
.cn.me{background:var(--card2);box-shadow:inset 0 0 0 2px var(--acc)}
.cn small{font-size:11px;font-weight:600;color:var(--ink2)}
.cn.me small{color:var(--accInk)}
.cn b{font-size:14px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.cn span{font:400 11px Arimo,Arial,sans-serif;color:var(--ink2);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.cl{width:84px;flex:none;display:flex;flex-direction:column;align-items:center;gap:2px;font:700 12px Arimo,Arial,sans-serif}
.cl i{height:2px;width:70px}
.adm{font-size:10px;font-weight:700;color:var(--accInk);border:1px solid var(--accInk);border-radius:5px;padding:1px 5px;white-space:nowrap}
.pcard{border-radius:14px;background:var(--card);padding:16px;display:flex;flex-direction:column;gap:10px;min-height:170px;min-width:0}
.pcard .ph{display:flex;align-items:center;gap:8px}
.pcard .ph b{font-size:15px;font-weight:600;flex:1}
.pcard .acts{margin-top:auto;display:flex;gap:8px;flex-wrap:wrap;align-items:center}
.spin{width:14px;height:14px;border-radius:50%;border:2px solid var(--card2);border-top-color:var(--acc);animation:spin .8s linear infinite;flex:none}
.pv{border-radius:10px;background:#F4F1EA;color:#1D2A33;padding:18px;display:flex;flex-direction:column;gap:10px;min-height:300px}
.pv .pt{font-size:15px;font-weight:700}
.pv .ph{font-size:22px;line-height:27px;font-weight:700}
.pv .ps{font-size:12px;line-height:17px;font-weight:500;color:#4A5A66}
.pv .pb{align-self:flex-start;border-radius:6px;background:#1D2A33;color:#F4F1EA;padding:8px 12px;font-size:12px;font-weight:600}
.pbar{height:6px;border-radius:3px;background:var(--card2);overflow:hidden}
.pbar i{display:block;height:100%;width:40%;background:var(--acc);border-radius:3px;animation:aInd 1.2s linear infinite}
.g3f{display:grid;grid-template-columns:minmax(0,2fr) minmax(0,1fr) minmax(0,1fr);gap:10px}
.dirty{min-height:60px;flex:none;border-top:1px solid var(--line);background:var(--card);display:flex;align-items:center;gap:12px;padding:10px 24px}
.dirty .dtext{font-size:13px;font-weight:600;flex:1;min-width:0}
.scrim{position:fixed;left:0;top:0;right:0;bottom:0;background:var(--scrim);display:flex;z-index:40;animation:aFade .2s both}
.scrim.mid{background:var(--scrim2);z-index:50;padding:16px;overflow:auto}
.drawer{width:520px;max-width:100%;height:100%;margin-left:auto;background:var(--card);border-left:1px solid var(--line);display:flex;flex-direction:column;animation:aSlide .22s cubic-bezier(.2,0,.2,1) both}
.dh{padding:18px 20px 12px;display:flex;align-items:flex-start;gap:10px}
.dh .tt b{font-size:20px;font-weight:600;overflow-wrap:anywhere}
.ib.x{width:34px;height:34px;background:var(--card2);color:var(--ink);font-size:14px}
.tabs{display:flex;gap:4px;padding:0 20px;border-bottom:1px solid var(--line);overflow-x:auto;flex:none}
.tab{height:40px;border:0;background:transparent;color:var(--ink2);font-size:13px;font-weight:600;padding:0 10px;cursor:pointer;white-space:nowrap}
.tab.on{color:var(--ink);box-shadow:inset 0 -2px 0 var(--acc)}
.dbody{flex:1;overflow:auto;padding:16px 20px;display:flex;flex-direction:column;gap:12px}
.dfoot{border-top:1px solid var(--line);padding:14px 20px;display:flex;gap:10px;align-items:center;flex-wrap:wrap}
.hist{display:flex;gap:10px;font-size:13px;padding:8px 0;border-top:1px solid var(--line)}
.hist .num{font-size:12px;color:var(--ink2);width:110px;flex:none}
.modal{width:460px;max-width:100%;border-radius:16px;background:var(--card);box-shadow:var(--shadow);padding:22px;display:flex;flex-direction:column;gap:12px;margin:auto;animation:aRise .25s both}
.modal.wide{width:540px}
.modal .mt{font-size:18px;font-weight:600}
.modal .mx{font-size:13px;line-height:19px;font-weight:500;color:var(--ink2);white-space:pre-line}
.mbtns{display:flex;justify-content:flex-end;gap:8px;margin-top:4px;flex-wrap:wrap}
.err{font-size:12px;font-weight:600;color:var(--accInk);white-space:pre-line}
.qr{display:block;margin:0 auto;width:220px;height:220px;border-radius:14px;background:#fff;padding:10px}
.hops{display:flex;flex-direction:column;gap:6px}
.hop{display:flex;gap:8px;align-items:center;font-size:13px}
.hop .pill{background:var(--card2);color:var(--ink)}
.toast{position:fixed;left:50%;bottom:24px;transform:translateX(-50%);border-radius:12px;background:var(--card2);box-shadow:var(--shadow);padding:12px 16px;display:none;gap:10px;align-items:center;font-size:13px;font-weight:600;z-index:70;white-space:pre-line;max-width:calc(100% - 32px);animation:aRise .2s both}
.toast.show{display:flex}
.toast .dot{background:var(--ok)}
.toast.bad .dot{background:var(--acc)}
.boot{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:14px;color:var(--ink2);font-size:13px;font-weight:500;padding:24px;text-align:center}

/* конструктор каскада */
.gsize{flex:none;overflow:hidden}
.gcanvas{position:relative;transform-origin:0 0}
.gcanvas svg{position:absolute;left:0;top:0;overflow:visible}
.glabel{position:absolute;width:76px;text-align:center;background:var(--bg);border:1px solid var(--card2);border-radius:9px;padding:3px 0;font-size:11px;line-height:1.3;z-index:2}
.glabel b{font-family:Arimo,Arial,sans-serif}
.gnode{position:absolute;background:var(--card);border-radius:14px;border:2px solid var(--card2);box-shadow:var(--shadow);animation:aRise .3s both}
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
.builder{position:fixed;left:0;top:0;right:0;bottom:0;background:var(--bg);display:none;flex-direction:column;z-index:35;animation:aFade .25s both}
.builder.show{display:flex}
.bbar{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;padding:14px 24px;border-bottom:1px solid var(--line)}
.bbar h2{margin:0;font-size:18px;font-weight:600}.bbar span.s{font-size:12px;color:var(--ink2)}
.bbtns{display:flex;gap:8px;flex-wrap:wrap;align-items:center}
.seg{display:flex;background:var(--card);border-radius:12px;padding:3px}
.seg button{min-height:32px;padding:0 12px;border:0;border-radius:9px;font-size:12px;font-weight:600;cursor:pointer;background:transparent;color:var(--ink)}
.seg button.on{background:var(--acc);color:var(--onacc)}
.bbody{flex:1;min-height:0;display:flex;flex-direction:row;overflow:auto}
.bbody aside{flex:none;padding:14px;display:flex;flex-direction:column;gap:8px}
.bbody aside.pal{width:230px;border-right:1px solid var(--line)}
.bbody aside.ins{width:270px;border-left:1px solid var(--line);gap:10px}
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
.irw{display:flex;justify-content:space-between;gap:8px;background:var(--card);border-radius:12px;padding:8px 12px;font-size:12px}
.irw span{color:var(--ink2)}.irw b{font-family:Arimo,Arial,sans-serif;text-align:right;word-break:break-all}
.ibox{background:var(--card);border-radius:12px;padding:10px 12px}
.ibox small{font-size:11px;color:var(--ink2)}
.bars{display:flex;align-items:flex-end;gap:3px;height:48px;margin-top:6px}
.bars div{flex:1;border-radius:2px;background:var(--ink2)}
.bars div.last{background:var(--acc)}.bars div.fail{background:var(--accInk)}
.rbox{font-size:12px;line-height:1.5}.rbox .k{color:var(--ink2)}.rbox .v{font-weight:600}.rbox .n{font-family:Arimo,Arial,sans-serif;margin-top:4px}

@media(max-width:1199px){.search{width:200px}.g5{grid-template-columns:repeat(3,minmax(0,1fr))}.g3{grid-template-columns:repeat(2,minmax(0,1fr))}.gcv{grid-template-columns:minmax(0,1fr)}}
@media(max-width:899px){
.app{flex-direction:column;height:auto;min-height:100vh}
.side{width:auto;border-right:0;border-bottom:1px solid var(--line)}
.brand{padding:10px 16px}
.mbtn{display:flex!important}
.side .nodesw,.side nav.nav,.side .sfoot{display:none}
.side.open .nodesw{display:block}
.side.open nav.nav{display:flex;padding-bottom:10px}
.side.open .sfoot{display:flex}
header.top{height:auto;flex-wrap:wrap;padding:12px 16px;gap:10px}
.crumb{flex:1}
header.top>.grow{display:none}
.search{order:9;width:100%}
.notice{margin:12px 16px 0}
main{overflow:visible;padding:16px}
.g2,.g21,.g141,.g3,.gcv,.g3f{grid-template-columns:minmax(0,1fr)}
.g5{grid-template-columns:repeat(2,minmax(0,1fr))}
.chain{flex-direction:column;align-items:stretch}
.cw{flex-direction:column;align-items:stretch}
.cl{width:auto;height:40px;justify-content:center;flex-direction:row;gap:8px}
.cl i{width:2px;height:24px}
.dirty{position:sticky;bottom:0;padding:10px 16px;flex-wrap:wrap}
.drawer{width:100%;border-left:0}
.lrow .lname{width:110px}
.bbody{flex-direction:column}
.bbody aside.pal,.bbody aside.ins{width:100%;border:0}
.bbody aside.pal{border-bottom:1px solid var(--line)}
.bbody aside.ins{border-top:1px solid var(--line)}
}
@media(max-width:539px){.g5{grid-template-columns:minmax(0,1fr)}.health span.ht{display:none}.bbar{padding:14px}}
</style>
</head>
<body>
<div id="app" class="app"></div>

<div class="builder" id="builder">
<div class="bbar">
<div style="min-width:0"><h2 data-i="builder"></h2><span class="s" data-i="builderHint"></span></div>
<div class="bbtns">
<div class="seg" id="b-modes"></div>
<button type="button" class="b lg" id="b-check" data-i="check"></button>
<button type="button" class="b lg acc" id="b-apply" data-i="builderApply"></button>
<button type="button" class="b lg" id="b-close" aria-label="✕">✕</button>
</div></div>
<div class="bbody">
<aside class="pal"><span class="cap" data-i="available"></span><div id="b-pal" style="display:flex;flex-direction:column;gap:8px"></div><button type="button" class="b lg" id="b-byKey" data-i="byKey"></button><button type="button" class="b lg" id="b-issueKey" data-i="issueKey"></button></aside>
<div class="canvas"><div class="gsize" id="b-size"><div class="gcanvas" id="b-canvas"></div></div><div class="warns" id="b-warns"></div></div>
<aside class="ins"><span class="cap" data-i="inspector"></span><strong class="t" id="i-title"></strong>
<div id="i-rows" style="display:flex;flex-direction:column;gap:6px"></div>
<div class="ibox"><small id="i-histcap"></small><div class="bars" id="i-bars"></div></div>
<div class="ibox rbox"><div class="k" data-i="route"></div><div class="v" id="i-route"></div><div class="n" id="i-total"></div></div>
</aside>
</div></div>

<div class="toast" id="toast" role="status"><span class="dot"></span><span id="toast-t"></span></div>

<script>
const H={'X-Panel-Request':'1','Content-Type':'application/json'};const $=id=>document.getElementById(id);
let lang=document.documentElement.lang==='en'?'en':'ru',theme=document.documentElement.dataset.theme==='light'?'light':'dark';
const D={ru:{
gMon:'Мониторинг',gAcc:'Доступ',gNet:'Сеть',gSrv:'Сервер',
p_overview:'Обзор',p_sessions:'Сессии',p_audit:'Журнал событий',p_clients:'Клиенты',p_links:'Связи сетей',p_cascade:'Каскад',p_protocols:'Протоколы',p_cover:'Сайт-прикрытие',p_settings:'Настройки',p_security:'Безопасность',
navL:'Разделы панели',menu:'Меню',nodeL:'Узел',nodesL:'Узлы каскада',thisServer:'Этот сервер',thisServerSub:'панель открыта здесь',viaRoute:'через {0}',
search:'Поиск',searchPh:'Клиент, адрес, подсеть…',addClient:'+ Клиент',logout:'Выйти',loginPw:'вход по паролю',loginRole:'вход по роли клиента',
themeDark:'Тёмная тема',themeLight:'Светлая тема',langSwitch:'English',
loading:'Загрузка…',retry:'Повторить',wait:'Подождите…',cancel:'Отмена',close:'Закрыть',save:'Сохранить',apply:'Применить',discard:'Отменить',create:'Создать',done:'Готово',copy:'Скопировать',copied:'Скопировано',download:'Скачать файл',open:'Открыть',enable:'Включить',
errN:'Ошибка {0}',bootErr:'Не удалось загрузить данные панели: {0}',
nodeNote:'Панель узла «{0}» открыта через каскад: изменения применяются к этому узлу.',downTitle:'Узел «{0}» не отвечает',downText:'Данные этого узла недоступны, пока связь не восстановится.',switching:'Подключаемся к узлу «{0}»…',stillDown:'Узел по-прежнему не отвечает',
hOk:'Всё работает',hWarn:'Предупреждений: {0}',hBad:'Проблем: {0}',
units:['Б','КБ','МБ','ГБ','ТБ'],bunits:['бит/с','Кбит/с','Мбит/с','Гбит/с'],ms:'мс',dd:'д',hh:'ч',mm:'мин',
agoS:'{0} с назад',agoM:'{0} мин назад',agoH:'{0} ч назад',agoD:'{0} дн назад',now:'сейчас',
kClients:'Клиентов',kClientsSub:'админов: {0} · заблокировано: {1}',kOnline:'Сейчас в сети',kTraffic:'Трафик за сутки',kTrafficAll:'Трафик с запуска',kSinceStart:'с запуска сервера',kSites:'Сайты (шлюзы)',kGwOff:'не в сети: {0}',kGwOk:'все шлюзы в сети',kGwNone:'шлюзов нет',kUptime:'Работает',kVer:'версия {0}',
trTitle:'Трафик за 24 часа',rxS:'получено',txS:'отправлено',trNA:'Почасовая статистика недоступна на этой версии сервера.',trPeak:'Пик в {0}: {1} за час',
attn:'Требует внимания',allOk:'Всё в порядке: проблем не найдено.',aGwOff:'Шлюз «{0}» не в сети',aGwBlocked:'клиент заблокирован',aLast:'последний раз {0}',aBlocked:'{0} заблокирован после {1} ошибок входа',aUntil:'до {0}',aCert:'Сертификат домена не получен',aRepo:'Сайт-прикрытие не обновился',aUp:'Нет связи с узлом «{0}»',aCtr:'Контейнер {0}: {1}',
svcTitle:'Службы сервера',running:'работает',svcNA:'Список контейнеров недоступен.',
evTitle:'Последние события',evAll:'Весь журнал',evNA:'Журнал событий недоступен на этой версии сервера.',evNone:'Событий пока нет.',
k_all:'Все',k_login:'Вход',k_clients:'Клиенты',k_links:'Связи',k_cascade:'Каскад',k_settings:'Настройки',k_system:'Система',
evTime:'Время',evKind:'Тип',evWho:'Кто',evText:'Событие',evIp:'Адрес',export:'Экспорт',exportCsv:'Экспорт CSV',csvSaved:'CSV сохранён: {0}',
f_all:'Все',f_online:'В сети',f_gw:'Шлюзы',f_admin:'Администраторы',f_blocked:'Заблокированы',
selN:'Выбрано: {0}',block:'Заблокировать',unblock:'Разблокировать',del:'Удалить',clearSel:'Снять выбор',selAll:'Выбрать всех',selOne:'Выбрать {0}',
thClient:'Клиент',thRole:'Роль',thAddr:'Адрес в туннеле',thRoute:'Маршрут',thState:'Состояние',thTraffic:'Трафик ↓/↑',
gwTag:'шлюз',roleAdminS:'админ',roleUserS:'пользователь',roleNone:'без роли',roleTransit:'транзит',
stBlocked:'заблокирован',stOnline:'в сети',stOffline:'не в сети',
rDirect:'Напрямую',rVia:'через {0}',rAuto:'Авто → {0}',rTransit:'транзит от {0}',unreachable:'путь недоступен',
noRows:'Ничего не найдено. Сбросьте фильтр или измените запрос.',noClients:'Клиентов пока нет. Нажмите «+ Клиент».',shownOf:'Показано {0} из {1}',perPage:'Строк на странице: 25',pageOf:'стр. {0} из {1}',prev:'Назад',next:'Вперёд',
bulkBlockT:'Заблокировать клиентов: {0}?',bulkBlockX:'Они сразу перестанут проходить аутентификацию, открытые сессии будут разорваны.',bulkDelT:'Удалить клиентов: {0}?',bulkDelX:'Действие нельзя отменить: ключи будут удалены, выданные конфиги перестанут работать.\nДля подтверждения введите число клиентов.',
bulkBlocked:'Заблокировано клиентов: {0}',bulkUnblocked:'Разблокировано клиентов: {0}',bulkDeleted:'Удалено клиентов: {0}',bulkErr:'Не получилось для {0}: {1}',
needMatch:'Не совпадает — введите «{0}»',
sessHead:'Активных подключений: {0} · HTTP/3 — {1}, HTTP/2 — {2} · IPv6 — {3}',autoRef:'Обновлять каждые 5 с',
thTransport:'Транспорт',thFamily:'Семейство',thRemote:'Внешний адрес',thEntry:'Вход',thSince:'Начало',thRate:'Скорость ↓/↑',kill:'Разорвать',
entryDirect:'напрямую',entryRelay:'по ключу каскада ({0})',noSess:'Активных подключений нет.',
killT:'Разорвать сессию «{0}»?',killX:'Клиент переподключится автоматически. Чтобы не пускать его — заблокируйте.',killed:'Сессии разорваны',
sessNote:'Разрыв сессии не блокирует клиента: он переподключится через ~100 мс. Чтобы не пускать клиента — заблокируйте его на странице «Клиенты».',
lkC2C:'Клиент ↔ клиент',lkC2CH:'Устройства VPN видят друг друга по адресам туннеля (общие папки, RDP, SSH между своими).',lkC2S:'Клиент ↔ сайт',lkC2SH:'Устройства VPN попадают в сети за шлюзами, а хосты этих сетей — к устройствам VPN.',lkS2S:'Сайт ↔ сайт',lkS2SH:'Сети за разными шлюзами связаны между собой: офис ↔ склад.',
lkOn:'Разрешено · {0}',lkOff:'Запрещено · {0}',lkChanged:'Изменено — не применено',nClients:'клиентов: {0}',nNets:'сетей за шлюзами: {0}',nGws:'шлюзов: {0}',
sitesT:'Сайты — сети за шлюзами',assignGw:'Назначить шлюз',thNet:'Подсеть',thGw:'Шлюз',thGwOn:'Шлюз в сети',thSrvRoute:'Маршрут на сервере',routeOff:'снят (клиент заблокирован)',noSites:'Сетей за шлюзами нет. Укажите подсети у клиента-шлюза (вкладка «Сети»), на шлюзе: sudo veil connect -gateway',
probeT:'Проверка доступа',probeX:'Покажет, пропустит ли сервер трафик при текущих (ещё не применённых) настройках.',from:'Откуда',to:'Куда',
epClient:'Клиент VPN',epClient2:'Другой клиент VPN',epSite:'Сайт за «{0}»',epNet:'Интернет',
prSame:'Выберите разные точки',prPass:'Пропустит',prBlock:'Заблокирует',prDown:'Разрешено, но недоступно',prNat:'Выход в интернет через NAT сервера не зависит от связей',prRule:'Правило «{0}» разрешено',prRuleOff:'Правило «{0}» выключено',prGwDown:'Правило «{0}» разрешает, но шлюз «{1}» не в сети',
dirtyLinks:'Связи: изменений {0}. Применятся сразу ко всему трафику, открытые соединения не разрываются.',lkSaved:'Связи применены',unsaved:'Несохранённые изменения',
chainT:'Маршрут трафика',chainX:'Замер задержек раз в ~30 с · перестройка при выигрыше ≥20% и ≥5 мс',rebuild:'Перестроить сейчас',rebuilding:'Перестраиваю…',builderOpen:'Конструктор',
chainOf:'Маршрут клиента «{0}» · {1}',chainNone:'Каскад не настроен: трафик клиентов выходит прямо с этого сервера.',modeAuto:'авто по задержкам',modeManual:'вручную',
source:'источник',thisNode:'этот узел',thisNodeExit:'этот узел · выход',exit:'выход',inter:'промежуточный',onlineOf:'в сети {0} из {1}',
upsT:'Следующие узлы (куда отправляем)',upAdd:'+ По ключу',noUps:'Нет — этот сервер является выходом.',measuring:'замер…',noLink:'нет связи',disconnect:'Отключить',
relsT:'Выданные ключи (кто отправляет к нам)',relAdd:'+ Выдать ключ',noKeys:'Ключи не выдавались.',revoke:'Отозвать',admBadge:'админ каскада',relSub:'{0} · {1} · выдан {2}',online:'на связи',offline:'не на связи',
upDelT:'Отключить «{0}»?',upDelX:'Клиенты с маршрутом через этот узел потеряют выход. Автомаршрут попробует найти другой путь.',upDeleted:'Узел отключён',
relDelT:'Отозвать ключ «{0}»?',relDelX:'Узел, которому он выдан, сразу перестанет отправлять трафик через этот сервер. Все клиенты, идущие через этот узел, будут удалены.',relDeleted:'Ключ отозван',
upTitle:'Подключить следующий узел',upText:'Вставьте ключ veil-relay://, который выдал следующий узел.',nodeName:'Название узла',upKey:'Ключ каскада',connect:'Подключить',upDone:'Узел подключён, задержка замеряется',
relTitle:'Выдать ключ каскада',relText:'Ключ даёт предыдущему узлу право отправлять трафик через этот сервер. Показывается один раз.',relFor:'Кому: название узла',relAdminL:'Разрешить администрирование',relAdminSub:'Узел сможет открывать панель этого сервера через каскад',relAdminWarn:'Узел получает полный доступ к панели этого сервера: клиенты, каскад, ключи и пароль. Выдавайте такой ключ только своим серверам.',issue:'Выдать',
relKeyTitle:'Ключ каскада для «{0}»',relKeyWarn:'Ключ показывается один раз. Вставьте его в панели узла «{0}»: Каскад → «+ По ключу».',
optNone:'Маршруты актуальны: лучше на 20% вариантов нет (или новый путь ещё не подтвердился — нужно два замера подряд).',
protoIntro:'Каждый протокол — отдельный контейнер на этом сервере. Клиенты WireGuard, AmneziaWG, OpenVPN и IPsec выдаются здесь же; файлы конфигурации показываются один раз.',
protoNA:'Сведения о протоколах недоступны на этой версии сервера.',protoNoDocker:'Управление протоколами из панели выключено: панели не дан доступ к Docker. Управляйте ими из приложения.',
dVeil:'Свой протокол: HTTP/3 и HTTP/2 на 443, сайт-прикрытие, каскад',dAdmin:'Эта панель. Доступна только через VPN',dWG:'Быстрый и простой, IPv4 и IPv6 внутри туннеля',dAWG:'WireGuard с обфускацией Jc/S1/S2/H1–H4',dOVPN:'Совместим с любыми клиентами OpenVPN',dIPSEC:'Встроен в Windows, macOS, iOS',
port:'Порт',clientsL:'Клиентов',noPorts:'без портов',notInstalled:'не установлен',installFromApp:'Установка — из приложения Veil',restart:'Перезапустить',
rsT:'Перезапустить {0}?',rsX:'Подключения {0} прервутся на несколько секунд и восстановятся сами.',rsXAdmin:'Панель будет недоступна несколько секунд, вход сохранится.',rsXVeil:'Все подключения прервутся на несколько секунд и восстановятся сами.',rsDone:'{0} перезапущен',
pcT:'Новый клиент {0}',pcX:'Конфигурация будет показана один раз.',pcPh:'Имя клиента',pcTitle:'Конфигурация {0} для «{1}»',pcWarn:'Показывается один раз: сохраните файл или скопируйте текст сейчас.',
cvT:'Что видят посторонние на 443',cvGen:'Сгенерированный сайт-визитка',cvGenSub:'Выдуманная компания, создаётся при установке',cvRepo:'Свой сайт из репозитория',cvRepoSub:'Статика из GitHub/GitLab/Gitea, обновляется автоматически',
company:'Компания',domainL:'Домен',cvGenNote:'Сгенерировать другой сайт-визитку можно из приложения Veil.',cvBack:'Вернуть сгенерированный сайт',cvBackT:'Вернуть сгенерированный сайт?',cvBackX:'Посторонние сразу увидят сайт-визитку вместо сайта из репозитория. Настройки репозитория будут сброшены.',cvBackDone:'Восстановлен сгенерированный сайт',
srUrl:'Репозиторий',srRef:'Ветка',srDir:'Каталог',srToken:'Токен доступа (для закрытого репозитория)',srClrTok:'Сбросить токен',srClrT:'Сбросить токен?',srClrX:'Закрытый репозиторий перестанет загружаться, пока вы не укажете токен снова.',srClrDone:'Токен сброшен',
srSave:'Сохранить и загрузить',srRefresh:'Обновить сейчас',srBusy:'Идёт загрузка…',srErr:'Не удалось загрузить репозиторий: {0}',srOk:'Обновлено {0}',srMeta:'коммит {0} · файлов: {1}',srWait:'Ещё не загружен',srNeedUrl:'Укажите адрес репозитория',srQueued:'Обновление запущено',
pvT:'Предпросмотр',pvHead:'Надёжный партнёр для вашего бизнеса',pvSub:'Сайт-визитка, который видят все, кто заходит на сервер без ключа.',pvBtn:'Связаться',pvRepo:'Сайт из репозитория',
adrT:'Публичные адреса',adrX:'Попадают в новые конфиги клиентов и ключи каскада. Старые конфиги не меняются.',
domT:'Домен и сертификат',certNone:'Домен не задан: клиенты подключаются по IP с закреплённым сертификатом.',certPending:'Получаем сертификат…',certReady:'Let’s Encrypt: действует до {0}. Новые клиенты подключаются по домену.',certErr:'Ошибка сертификата',certAfter:'После применения сертификат будет выпущен автоматически (TLS-ALPN-01 на 443).',
frT:'Маскировка под чужой сайт (SNI)',frNone:'Без маскировки',frOther:'Свой домен…',frWarn:'После включения работает только HTTP/2 (UDP/443 закрывается), выданные конфиги и ключи каскада перестанут работать — выпустите их заново.',frOn:'Включена: посторонние соединения уходят на настоящий {0}, свои клиенты проходят в туннель.',frDomainNote:'При включённой маскировке сертификат домена не обновится: Let’s Encrypt не пройдёт проверку.',frBad:'Укажите доменное имя сайта, например ya.ru',
frAskT:'Включить маскировку под {0}?',frAskX:'UDP/443 закроется, все выданные конфиги и ключи каскада перестанут работать — их нужно выпустить заново.',
dirtySet:'Настройки: изменений {0}.',dirtyFront:'Маскировка требует перевыпуска конфигов.',setApplied:'Настройки применены',
mtT:'Обслуживание',version:'Версия',mode:'Режим',modeContainer:'Docker (контейнер veil-admin)',modeBuiltin:'встроенная панель veil',adminUrl:'Адрес панели',
backup:'Скачать резервную копию',bkT:'Скачать резервную копию?',bkX:'В файле — ключи сервера и всех клиентов. Храните его надёжно и никому не передавайте.',bkDone:'Резервная копия скачана: {0} — в ней ключи, храните надёжно',bkNA:'Резервная копия недоступна на этой версии сервера.',
updNote:'Обновление сервера и установка протоколов — из приложения Veil.',restartVeil:'Перезапустить veil',
pwT:'Пароль панели',pwCur:'Текущий пароль',pwNew:'Новый пароль',pwRep:'Ещё раз',pwHint:'Не короче 8 символов. Все остальные сессии панели будут завершены.',pwShort:'Новый пароль короче 8 символов',pwMismatch:'Пароли не совпадают',pwReady:'Готово к смене',pwSave:'Сменить пароль',pwDone:'Пароль изменён. Войдите заново.',pwRoleNote:'Вы вошли по роли клиента — текущий пароль не нужен.',
polT:'Политика доступа',polFrom:'Откуда',polFromV:'только из туннеля VPN; без VPN /admin — 404',polAdm:'Клиенты-«администраторы»',polAdmV:'входят без пароля: {0}',polUsr:'Клиенты-«пользователи»',polUsrV:'панель не видят',polFail:'Ошибки входа',polFailV:'5 за 15 минут — IP блокируется на 15 минут',polSess:'Сессия',polSessV:'12 часов, cookie HttpOnly, Secure, SameSite=Strict',none:'нет',
asT:'Открытые сессии панели',asKillAll:'Завершить все, кроме этой',asKill:'Завершить',asYou:'вы',asNA:'Список сессий недоступен на этой версии сервера.',asKilled:'Остальные сессии панели завершены',asKilled1:'Сессия завершена',
blT:'Заблокированные адреса',blNone:'Заблокированных адресов нет.',blSub:'{0} ошибок · до {1}',blNA:'Список блокировок недоступен на этой версии сервера.',blDone:'Адрес разблокирован',
tOverview:'Обзор',tConfig:'Конфигурация',tNets:'Сети (site)',tAccess:'Доступ',tHistory:'История',clientL:'Клиент {0}',
dState:'Состояние',dAddrs:'Адреса',dRoute:'Маршрут',dCreated:'Создан',dHs:'Рукопожатие',dTraffic:'Трафик',dTrafficV:'скачано {0}, отправлено {1}',dSess:'Сессии',
name:'Имя',namePh:'Например «Ольга — ноутбук»',note:'Заметка',gwSite:'шлюз сайта',transitNote:'Транзитный клиент узла «{0}»: управляется на том узле.',
cfgBox:'Ссылка veil:// и файл .veil содержат приватный ключ клиента и выдаются только один раз — сервер их не хранит. Если конфиг потерян или утёк, выпустите новый: старый сразу перестанет работать.',reissue:'Выпустить новый конфиг',
reT:'Выпустить новый конфиг для «{0}»?',reX:'Старый конфиг перестанет работать сразу. Новую ссылку покажем один раз.',newCfgTitle:'Новый конфиг «{0}»',newCfgWarn:'Старый конфиг больше не работает. Ссылка и QR показываются один раз: в них приватный ключ.',
netsX:'Если клиент — шлюз локальной сети, укажите её подсети (по одной в строке). Кто их видит — на странице «Связи сетей». На шлюзе:',netsWill:'Будут маршрутизироваться к этому клиенту после сохранения',netsEmpty:'Пусто — обычное устройство',
nets1:'«{0}» — не подсеть, нужно например 192.168.1.0/24',nets2:'«{0}» — не подсеть',nets3:'{0}: слишком широкая сеть',nets4:'{0} пересекается с туннелем {1}',
roleLabel:'Роль',roleUser:'Пользователь',roleUserSub:'Только VPN, панель не видит',roleAdmin:'Администратор',roleAdminSub:'Входит в панель без пароля, управляет сервером и каскадом',newAdminWarn:'Администратор получает полный доступ к панели этого сервера. Назначайте роль только тем, кому доверяете.',
blockedL:'Заблокирован',blockedSub:'Сразу не проходит аутентификацию; открытые сессии разрываются',blockedOn:'Клиент заблокирован, сессии разорваны',blockedOff:'Клиент разблокирован',
histNone:'Событий по этому клиенту нет.',histCreated:'Клиент создан',delClient:'Удалить клиента',
delT:'Удалить «{0}»?',delX:'Ключи удаляются, подключения разрываются{0}. Отменить нельзя.\nВведите имя клиента для подтверждения.',delNets:', сети {0} перестанут маршрутизироваться',deleted:'Клиент удалён',clientSaved:'Изменения клиента сохранены',
newClient:'Новый клиент',route:'Маршрут',modeManualOpt:'Вручную — выбрать узлы по порядку',modeAutoOpt:'Автоматически по задержкам',exitNode:'Выходной узел',hopsCount:'Промежуточных узлов',allAvail:'все доступные',
autoHint:'Вход и выход фиксированы, промежуточные узлы упорядочиваются по минимальной суммарной задержке и перестраиваются автоматически, когда другой путь стабильно быстрее на 20%. Конфиг клиента при этом не меняется.',
noCascadeRoute:'Каскад не настроен — трафик клиента выходит с этого сервера.',undo:'убрать',nextHop:'Следующий узел',exitOnLast:'выход на последнем узле',exitOnThis:'выход на этом сервере',nextThrough:'дальше через {0}',noMoreNodes:'дальше узлов нет — выход на последнем',
createdTitle:'Клиент «{0}» создан',createdWarn:'Ссылка и QR показываются один раз: в них приватный ключ. Отсканируйте QR в приложении Veil или передайте файл.',routeVia:'Маршрут: {0}',
builder:'Конструктор каскада',builderHint:'Вход и выход задаются явно. Двигайте узлы, добавляйте их из списка и создайте клиента с этим маршрутом.',builderApply:'Создать клиента',
check:'Проверить маршрут',available:'Доступные узлы',inspector:'Узел',byKey:'Узел по ключу…',issueKey:'Выдать ключ…',
clients:'Клиенты',stClients:'Клиентов',stOnline:'Онлайн',internet:'Интернет',entry:'вход',entryExit:'вход и выход',address:'Адрес',transport:'Транспорт',latency:'Задержка',status:'Состояние',addresses:'Адреса',via:'Выход через',
mManual:'Вручную',mAuto:'Авто по задержкам',moveEarlier:'Сдвинуть раньше',moveLater:'Сдвинуть позже',rmRoute:'Убрать из маршрута',addRoute:'Добавить в маршрут {0}',
role:'Роль',latFromThis:'Задержка от этого сервера',position:'Позиция',histCap:'Задержка этого сервера до узла, последние замеры',noHistory:'Замеров пока нет',
nodesShort:'узл.',routeTotalFirst:'до первого узла: {0}',routeTotalUnk:'задержка по цепочке не измеряется',
wMany:'Больше 5 узлов: задержка растёт с каждым переходом.',wAuto:'Вход и выход фиксированы. Промежуточные узлы упорядочиваются по минимальной задержке и перестраиваются, когда другой путь быстрее на 20% и на 5 мс два замера подряд.',wOff:'Нет связи с узлами: {0}.',wOk:'Маршрут корректен: все узлы на связи.',
checkOk:'Маршрут принят сервером, узлов: {0}',applyEmpty:'Добавьте хотя бы один узел'
},en:{
gMon:'Monitoring',gAcc:'Access',gNet:'Network',gSrv:'Server',
p_overview:'Overview',p_sessions:'Sessions',p_audit:'Event log',p_clients:'Clients',p_links:'Network links',p_cascade:'Cascade',p_protocols:'Protocols',p_cover:'Cover site',p_settings:'Settings',p_security:'Security',
navL:'Panel sections',menu:'Menu',nodeL:'Node',nodesL:'Cascade nodes',thisServer:'This server',thisServerSub:'the panel is open here',viaRoute:'via {0}',
search:'Search',searchPh:'Client, address, subnet…',addClient:'+ Client',logout:'Sign out',loginPw:'password sign-in',loginRole:'signed in by client role',
themeDark:'Dark theme',themeLight:'Light theme',langSwitch:'Русский',
loading:'Loading…',retry:'Retry',wait:'Please wait…',cancel:'Cancel',close:'Close',save:'Save',apply:'Apply',discard:'Discard',create:'Create',done:'Done',copy:'Copy',copied:'Copied',download:'Download file',open:'Open',enable:'Enable',
errN:'Error {0}',bootErr:'Could not load the panel data: {0}',
nodeNote:'The panel of node "{0}" is opened through the cascade: changes apply to that node.',downTitle:'Node "{0}" is not responding',downText:'The data of this node is unavailable until the link is restored.',switching:'Connecting to node "{0}"…',stillDown:'Still no answer from the node',
hOk:'All systems go',hWarn:'Warnings: {0}',hBad:'Problems: {0}',
units:['B','KB','MB','GB','TB'],bunits:['bit/s','Kbit/s','Mbit/s','Gbit/s'],ms:'ms',dd:'d',hh:'h',mm:'min',
agoS:'{0} s ago',agoM:'{0} min ago',agoH:'{0} h ago',agoD:'{0} d ago',now:'now',
kClients:'Clients',kClientsSub:'admins: {0} · blocked: {1}',kOnline:'Online now',kTraffic:'Traffic, 24 h',kTrafficAll:'Traffic since start',kSinceStart:'since the server started',kSites:'Sites (gateways)',kGwOff:'offline: {0}',kGwOk:'all gateways online',kGwNone:'no gateways',kUptime:'Uptime',kVer:'version {0}',
trTitle:'Traffic, last 24 hours',rxS:'received',txS:'sent',trNA:'Hourly statistics are not available on this server version.',trPeak:'Peak at {0}: {1} per hour',
attn:'Needs attention',allOk:'All good: no problems found.',aGwOff:'Gateway "{0}" is offline',aGwBlocked:'client is blocked',aLast:'last seen {0}',aBlocked:'{0} blocked after {1} failed sign-ins',aUntil:'until {0}',aCert:'Domain certificate not obtained',aRepo:'Cover site failed to update',aUp:'No link to node "{0}"',aCtr:'Container {0}: {1}',
svcTitle:'Server services',running:'running',svcNA:'The container list is unavailable.',
evTitle:'Recent events',evAll:'Full log',evNA:'The event log is not available on this server version.',evNone:'No events yet.',
k_all:'All',k_login:'Sign-in',k_clients:'Clients',k_links:'Links',k_cascade:'Cascade',k_settings:'Settings',k_system:'System',
evTime:'Time',evKind:'Type',evWho:'Who',evText:'Event',evIp:'Address',export:'Export',exportCsv:'Export CSV',csvSaved:'CSV saved: {0}',
f_all:'All',f_online:'Online',f_gw:'Gateways',f_admin:'Administrators',f_blocked:'Blocked',
selN:'Selected: {0}',block:'Block',unblock:'Unblock',del:'Delete',clearSel:'Clear selection',selAll:'Select all',selOne:'Select {0}',
thClient:'Client',thRole:'Role',thAddr:'Tunnel address',thRoute:'Route',thState:'State',thTraffic:'Traffic ↓/↑',
gwTag:'gateway',roleAdminS:'admin',roleUserS:'user',roleNone:'no role',roleTransit:'transit',
stBlocked:'blocked',stOnline:'online',stOffline:'offline',
rDirect:'Direct',rVia:'via {0}',rAuto:'Auto → {0}',rTransit:'transit from {0}',unreachable:'path unavailable',
noRows:'Nothing found. Reset the filter or change the query.',noClients:'No clients yet. Press "+ Client".',shownOf:'Showing {0} of {1}',perPage:'Rows per page: 25',pageOf:'page {0} of {1}',prev:'Back',next:'Next',
bulkBlockT:'Block {0} clients?',bulkBlockX:'They will stop authenticating at once; open sessions will be dropped.',bulkDelT:'Delete {0} clients?',bulkDelX:'This cannot be undone: keys are deleted and issued configs stop working.\nType the number of clients to confirm.',
bulkBlocked:'Clients blocked: {0}',bulkUnblocked:'Clients unblocked: {0}',bulkDeleted:'Clients deleted: {0}',bulkErr:'Failed for {0}: {1}',
needMatch:'Does not match: type "{0}"',
sessHead:'Active connections: {0} · HTTP/3 — {1}, HTTP/2 — {2} · IPv6 — {3}',autoRef:'Refresh every 5 s',
thTransport:'Transport',thFamily:'Family',thRemote:'Remote address',thEntry:'Entry',thSince:'Started',thRate:'Rate ↓/↑',kill:'Drop',
entryDirect:'direct',entryRelay:'by cascade key ({0})',noSess:'No active connections.',
killT:'Drop the session of "{0}"?',killX:'The client will reconnect automatically. To keep it out, block it.',killed:'Sessions dropped',
sessNote:'Dropping a session does not block the client: it reconnects in ~100 ms. To keep a client out, block it on the Clients page.',
lkC2C:'Client ↔ client',lkC2CH:'VPN devices see each other by tunnel addresses (shared folders, RDP, SSH between your own).',lkC2S:'Client ↔ site',lkC2SH:'VPN devices reach networks behind gateways, and hosts of those networks reach VPN devices.',lkS2S:'Site ↔ site',lkS2SH:'Networks behind different gateways are linked: office ↔ warehouse.',
lkOn:'Allowed · {0}',lkOff:'Denied · {0}',lkChanged:'Changed — not applied',nClients:'clients: {0}',nNets:'networks behind gateways: {0}',nGws:'gateways: {0}',
sitesT:'Sites — networks behind gateways',assignGw:'Assign a gateway',thNet:'Subnet',thGw:'Gateway',thGwOn:'Gateway online',thSrvRoute:'Route on the server',routeOff:'removed (client blocked)',noSites:'No networks behind gateways. Set subnets on the gateway client ("Networks" tab); on the gateway run: sudo veil connect -gateway',
probeT:'Access check',probeX:'Shows whether the server would pass traffic with the current (not yet applied) settings.',from:'From',to:'To',
epClient:'VPN client',epClient2:'Another VPN client',epSite:'Site behind "{0}"',epNet:'Internet',
prSame:'Choose two different points',prPass:'Will pass',prBlock:'Will block',prDown:'Allowed but unreachable',prNat:'Internet access through the server NAT does not depend on links',prRule:'Rule "{0}" is allowed',prRuleOff:'Rule "{0}" is off',prGwDown:'Rule "{0}" allows it, but gateway "{1}" is offline',
dirtyLinks:'Links: {0} changes. They apply to all traffic at once; open connections are kept.',lkSaved:'Links applied',unsaved:'Unsaved changes',
chainT:'Traffic route',chainX:'Latency measured every ~30 s · rebuilt when ≥20% and ≥5 ms faster',rebuild:'Rebuild now',rebuilding:'Rebuilding…',builderOpen:'Builder',
chainOf:'Route of client "{0}" · {1}',chainNone:'No cascade configured: client traffic leaves directly from this server.',modeAuto:'auto by latency',modeManual:'manual',
source:'source',thisNode:'this node',thisNodeExit:'this node · exit',exit:'exit',inter:'intermediate',onlineOf:'online {0} of {1}',
upsT:'Next nodes (where we send)',upAdd:'+ By key',noUps:'None: this server is the exit.',measuring:'measuring…',noLink:'no link',disconnect:'Disconnect',
relsT:'Issued keys (who sends to us)',relAdd:'+ Issue key',noKeys:'No keys issued.',revoke:'Revoke',admBadge:'cascade admin',relSub:'{0} · {1} · issued {2}',online:'online',offline:'offline',
upDelT:'Disconnect "{0}"?',upDelX:'Clients routed through this node lose their exit. Auto-routing will try to find another path.',upDeleted:'Node disconnected',
relDelT:'Revoke key "{0}"?',relDelX:'The node it was issued to stops sending traffic through this server at once. All clients going through that node will be deleted.',relDeleted:'Key revoked',
upTitle:'Connect the next node',upText:'Paste the veil-relay:// key issued by the next node.',nodeName:'Node name',upKey:'Cascade key',connect:'Connect',upDone:'Node connected, measuring latency',
relTitle:'Issue a cascade key',relText:'The key lets the previous node send traffic through this server. It is shown once.',relFor:'For: node name',relAdminL:'Allow administration',relAdminSub:'The node can open this server’s panel through the cascade',relAdminWarn:'The node gets full access to this server’s panel: clients, cascade, keys and password. Issue such a key only to your own servers.',issue:'Issue',
relKeyTitle:'Cascade key for "{0}"',relKeyWarn:'The key is shown once. Paste it in the panel of node "{0}": Cascade → "+ By key".',
optNone:'Routes are up to date: nothing 20% better (or the new path is not confirmed yet: two checks in a row are needed).',
protoIntro:'Each protocol is a separate container on this server. WireGuard, AmneziaWG, OpenVPN and IPsec clients are issued here; configuration files are shown once.',
protoNA:'Protocol information is not available on this server version.',protoNoDocker:'Protocol management from the panel is off: the panel has no access to Docker. Manage protocols from the app.',
dVeil:'Our protocol: HTTP/3 and HTTP/2 on 443, cover site, cascade',dAdmin:'This panel. Reachable only through the VPN',dWG:'Fast and simple, IPv4 and IPv6 inside the tunnel',dAWG:'WireGuard with Jc/S1/S2/H1–H4 obfuscation',dOVPN:'Works with any OpenVPN client',dIPSEC:'Built into Windows, macOS, iOS',
port:'Port',clientsL:'Clients',noPorts:'no ports',notInstalled:'not installed',installFromApp:'Install from the Veil app',restart:'Restart',
rsT:'Restart {0}?',rsX:'{0} connections drop for a few seconds and recover on their own.',rsXAdmin:'The panel is unavailable for a few seconds; you stay signed in.',rsXVeil:'All connections drop for a few seconds and recover on their own.',rsDone:'{0} restarted',
pcT:'New {0} client',pcX:'The configuration is shown once.',pcPh:'Client name',pcTitle:'{0} configuration for "{1}"',pcWarn:'Shown once: save the file or copy the text now.',
cvT:'What strangers see on 443',cvGen:'Generated business-card site',cvGenSub:'A made-up company, created at install time',cvRepo:'Your own site from a repository',cvRepoSub:'Static files from GitHub/GitLab/Gitea, updated automatically',
company:'Company',domainL:'Domain',cvGenNote:'A different business-card site can be generated from the Veil app.',cvBack:'Return to the generated site',cvBackT:'Return to the generated site?',cvBackX:'Strangers will see the business-card site instead of the repository site at once. Repository settings will be reset.',cvBackDone:'The generated site is restored',
srUrl:'Repository',srRef:'Branch',srDir:'Directory',srToken:'Access token (for a private repository)',srClrTok:'Reset token',srClrT:'Reset the token?',srClrX:'A private repository will stop loading until you enter the token again.',srClrDone:'Token reset',
srSave:'Save and load',srRefresh:'Update now',srBusy:'Loading…',srErr:'Could not load the repository: {0}',srOk:'Updated {0}',srMeta:'commit {0} · files: {1}',srWait:'Not loaded yet',srNeedUrl:'Enter the repository address',srQueued:'Update started',
pvT:'Preview',pvHead:'A reliable partner for your business',pvSub:'The business-card site seen by everyone who visits the server without a key.',pvBtn:'Contact us',pvRepo:'Site from the repository',
adrT:'Public addresses',adrX:'Go into new client configs and cascade keys. Existing configs do not change.',
domT:'Domain and certificate',certNone:'No domain set: clients connect by IP with a pinned certificate.',certPending:'Getting the certificate…',certReady:'Let’s Encrypt: valid until {0}. New clients connect by domain.',certErr:'Certificate error',certAfter:'After applying, the certificate is issued automatically (TLS-ALPN-01 on 443).',
frT:'Masquerade as another site (SNI)',frNone:'No masquerading',frOther:'Other domain…',frWarn:'Once enabled only HTTP/2 works (UDP/443 closes); issued configs and cascade keys stop working — issue them again.',frOn:'On: strangers are sent to the real {0}, your clients get through to the tunnel.',frDomainNote:'While masquerading is on, the domain certificate will not renew: Let’s Encrypt validation cannot pass.',frBad:'Enter a site domain name, for example ya.ru',
frAskT:'Masquerade as {0}?',frAskX:'UDP/443 closes; all issued configs and cascade keys stop working and must be issued again.',
dirtySet:'Settings: {0} changes.',dirtyFront:'Masquerading requires reissuing configs.',setApplied:'Settings applied',
mtT:'Maintenance',version:'Version',mode:'Mode',modeContainer:'Docker (veil-admin container)',modeBuiltin:'built-in veil panel',adminUrl:'Panel address',
backup:'Download backup',bkT:'Download a backup?',bkX:'The file contains the keys of the server and all clients. Keep it safe and never share it.',bkDone:'Backup downloaded: {0} — it contains keys, keep it safe',bkNA:'Backup is not available on this server version.',
updNote:'Server updates and protocol installation are done from the Veil app.',restartVeil:'Restart veil',
pwT:'Panel password',pwCur:'Current password',pwNew:'New password',pwRep:'Repeat',pwHint:'At least 8 characters. All other panel sessions will be ended.',pwShort:'The new password is shorter than 8 characters',pwMismatch:'Passwords do not match',pwReady:'Ready to change',pwSave:'Change password',pwDone:'Password changed. Sign in again.',pwRoleNote:'You signed in by client role: the current password is not needed.',
polT:'Access policy',polFrom:'From where',polFromV:'only from the VPN tunnel; without VPN /admin is 404',polAdm:'"Administrator" clients',polAdmV:'sign in without a password: {0}',polUsr:'"User" clients',polUsrV:'do not see the panel',polFail:'Failed sign-ins',polFailV:'5 in 15 minutes — the IP is blocked for 15 minutes',polSess:'Session',polSessV:'12 hours, HttpOnly, Secure, SameSite=Strict cookie',none:'none',
asT:'Open panel sessions',asKillAll:'End all except this one',asKill:'End',asYou:'you',asNA:'The session list is not available on this server version.',asKilled:'Other panel sessions ended',asKilled1:'Session ended',
blT:'Blocked addresses',blNone:'No blocked addresses.',blSub:'{0} failures · until {1}',blNA:'The block list is not available on this server version.',blDone:'Address unblocked',
tOverview:'Overview',tConfig:'Configuration',tNets:'Networks (site)',tAccess:'Access',tHistory:'History',clientL:'Client {0}',
dState:'State',dAddrs:'Addresses',dRoute:'Route',dCreated:'Created',dHs:'Handshake',dTraffic:'Traffic',dTrafficV:'downloaded {0}, uploaded {1}',dSess:'Sessions',
name:'Name',namePh:'For example "Olga — laptop"',note:'Note',gwSite:'site gateway',transitNote:'Transit client of node "{0}": managed on that node.',
cfgBox:'The veil:// link and the .veil file contain the client’s private key and are issued only once — the server does not keep them. If a config is lost or leaked, issue a new one: the old one stops working at once.',reissue:'Issue a new config',
reT:'Issue a new config for "{0}"?',reX:'The old config stops working at once. The new link is shown once.',newCfgTitle:'New config for "{0}"',newCfgWarn:'The old config no longer works. The link and QR are shown once and contain a private key.',
netsX:'If the client is a LAN gateway, list its subnets (one per line). Who can reach them is set on the Network links page. On the gateway:',netsWill:'Will be routed to this client after saving',netsEmpty:'Empty — a regular device',
nets1:'"{0}" is not a subnet; use e.g. 192.168.1.0/24',nets2:'"{0}" is not a subnet',nets3:'{0}: the network is too wide',nets4:'{0} overlaps the tunnel {1}',
roleLabel:'Role',roleUser:'User',roleUserSub:'VPN only, does not see the panel',roleAdmin:'Administrator',roleAdminSub:'Opens the panel without a password, manages the server and cascade',newAdminWarn:'An administrator gets full access to this server’s panel. Give the role only to people you trust.',
blockedL:'Blocked',blockedSub:'Stops authenticating at once; open sessions are dropped',blockedOn:'Client blocked, sessions dropped',blockedOff:'Client unblocked',
histNone:'No events for this client.',histCreated:'Client created',delClient:'Delete client',
delT:'Delete "{0}"?',delX:'Keys are deleted and connections dropped{0}. This cannot be undone.\nType the client name to confirm.',delNets:'; networks {0} stop being routed',deleted:'Client deleted',clientSaved:'Client changes saved',
newClient:'New client',route:'Route',modeManualOpt:'Manual: choose the nodes in order',modeAutoOpt:'Automatic by latency',exitNode:'Exit node',hopsCount:'Intermediate nodes',allAvail:'all available',
autoHint:'Entry and exit are fixed, intermediate nodes are ordered by the lowest total latency and rebuilt automatically when another path is consistently 20% faster. The client config does not change.',
noCascadeRoute:'No cascade configured: client traffic leaves from this server.',undo:'remove',nextHop:'Next node',exitOnLast:'exit on the last node',exitOnThis:'exit on this server',nextThrough:'next through {0}',noMoreNodes:'no more nodes: exit on the last one',
createdTitle:'Client "{0}" created',createdWarn:'The link and QR are shown once and contain a private key. Scan the QR in the Veil app or pass the file on.',routeVia:'Route: {0}',
builder:'Cascade builder',builderHint:'Entry and exit are chosen explicitly. Move nodes, add them from the list and create a client with this route.',builderApply:'Create a client',
check:'Check the route',available:'Available nodes',inspector:'Node',byKey:'Node by key…',issueKey:'Issue a key…',
clients:'Clients',stClients:'Clients',stOnline:'Online',internet:'Internet',entry:'entry',entryExit:'entry and exit',address:'Address',transport:'Transport',latency:'Latency',status:'Status',addresses:'Addresses',via:'Exit via',
mManual:'Manual',mAuto:'Auto by latency',moveEarlier:'Move earlier',moveLater:'Move later',rmRoute:'Remove from the route',addRoute:'Add to the route {0}',
role:'Role',latFromThis:'Latency from this server',position:'Position',histCap:'Latency from this server to the node, last checks',noHistory:'No checks yet',
nodesShort:'nodes',routeTotalFirst:'to the first node: {0}',routeTotalUnk:'chain latency is not measured',
wMany:'More than 5 nodes: the latency grows with every hop.',wAuto:'Entry and exit are fixed. Intermediate nodes are ordered by the lowest latency and rebuilt when another path is faster by 20% and 5 ms twice in a row.',wOff:'No connection to nodes: {0}.',wOk:'The route is valid: all nodes are online.',
checkOk:'The server accepted the route, nodes: {0}',applyEmpty:'Add at least one node'
}};
function t(k){let s=D[lang][k];if(s===undefined)s=D.ru[k];if(s===undefined)return k;if(typeof s!=='string')return s;for(let i=1;i<arguments.length;i++)s=s.split('{'+(i-1)+'}').join(arguments[i]);return s}
function store(k,v){try{localStorage.setItem(k,v)}catch(e){}}
function loc(){return lang==='ru'?'ru-RU':'en-GB'}
function num(s){return lang==='ru'?String(s).replace('.',','):String(s)}
function bytes(n){n=+n||0;const u=t('units');let i=0;while(n>=1024&&i<u.length-1){n/=1024;i++}return num(i?String(+n.toFixed(1)):String(Math.round(n)))+' '+u[i]}
function bits(n){n=+n||0;const u=t('bunits');let i=0;while(n>=1000&&i<u.length-1){n/=1000;i++}return num(i&&n<10?String(+n.toFixed(1)):String(Math.round(n)))+' '+u[i]}
function okDate(x){if(!x)return null;const d=new Date(x);return isNaN(d)||d.getFullYear()<2000?null:d}
function ago(x){const d=okDate(x);if(!d)return '—';const s=Math.max(0,(Date.now()-d)/1000);if(s<60)return t('agoS',Math.floor(s));if(s<3600)return t('agoM',Math.floor(s/60));if(s<86400)return t('agoH',Math.floor(s/3600));return t('agoD',Math.floor(s/86400))}
function dur(x){const d0=okDate(x);if(!d0)return '—';const s=Math.max(0,(Date.now()-d0)/1000);const d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60);return d?d+' '+t('dd')+' '+h+' '+t('hh'):(h?h+' '+t('hh')+' '+m+' '+t('mm'):m+' '+t('mm'))}
function ms1(v,d){return num((+v).toFixed(d||0))+' '+t('ms')}
function hm(d){return d.toLocaleTimeString(loc(),{hour:'2-digit',minute:'2-digit'})}
function fdt(x,short){const d=okDate(x);if(!d)return '—';if(short&&d.toDateString()===new Date().toDateString())return hm(d);return d.toLocaleDateString(loc(),{day:'2-digit',month:'2-digit'})+' '+hm(d)}
function fdate(x){const d=okDate(x);return d?d.toLocaleDateString(loc()):'—'}
function today(){return new Date().toISOString().slice(0,10)}

/* ---------- DOM ---------- */
function h(tag,p){const e=document.createElement(tag);if(p)for(const k in p){const v=p[k];if(v===null||v===undefined||v===false)continue;if(k.slice(0,2)==='on')e[k]=v;else if(k==='class')e.className=v;else if(k==='style')e.style.cssText=v;else if(k==='value')e.value=v;else e.setAttribute(k,v===true?'':String(v))}add(e,Array.prototype.slice.call(arguments,2));return e}
function add(e,kids){for(const c of kids){if(c===null||c===undefined||c===false)continue;if(Array.isArray(c))add(e,c);else e.append(c&&c.nodeType?c:document.createTextNode(String(c)))}}
function el(tag,cls,text){const e=document.createElement(tag);if(cls)e.className=cls;if(text!==undefined)e.textContent=text;return e}
function btn(text,cls,fn){const b=el('button',cls,text);b.type='button';b.onclick=fn;return b}
const IC={
chev:'<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M6 9l6 6 6-6"/></svg>',
check:'<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="4"><path d="M4 12l5 5 11-11"/></svg>',
check14:'<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M4 12l5 5 11-11"/></svg>',
search:'<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/></svg>',
logout:'<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 3h4a2 2 0 012 2v14a2 2 0 01-2 2h-4M10 17l5-5-5-5M15 12H3"/></svg>',
menu:'<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M4 7h16M4 12h16M4 17h16"/></svg>',
sun:'<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="4.5"/><path d="M12 2v2.5M12 19.5V22M2 12h2.5M19.5 12H22M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M4.9 19.1l1.8-1.8M17.3 6.7l1.8-1.8"/></svg>',
moon:'<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 14.5A8 8 0 019.5 4a8 8 0 1010.5 10.5z"/></svg>',
logo:'<svg width="26" height="26" viewBox="0 0 32 32"><path d="M3 4h8l9 23h-8z" fill="#FF393D"/><path d="M29 4h-8l-9 23h8z" fill="#6B6F78"/><path d="M16 16.8L12 27h8z" fill="#D9DBE0"/></svg>'};
function ico(n){const s=document.createElement('span');s.className='ico';s.setAttribute('aria-hidden','true');s.innerHTML=IC[n]||'';return s}
function dot(c,cls){return h('span',{class:'dot'+(cls?' '+cls:''),style:c?'background:'+c:null})}
let toastT=0;function flash(msg,bad,len){const e=$('toast');$('toast-t').textContent=msg;e.classList.toggle('bad',!!bad);e.classList.remove('show');void e.offsetWidth;e.classList.add('show');clearTimeout(toastT);toastT=setTimeout(()=>e.classList.remove('show'),len||(bad?5000:2600))}
function download(name,data,type){const a=document.createElement('a');a.href=URL.createObjectURL(data instanceof Blob?data:new Blob([data],{type:type||'application/octet-stream'}));a.download=name;document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(a.href),2000)}
function csv(rows){return String.fromCharCode(0xFEFF)+rows.map(r=>r.map(v=>{v=v===null||v===undefined?'':String(v);return /[",;\n\r]/.test(v)?'"'+v.replace(/"/g,'""')+'"':v}).join(',')).join('\r\n')+'\r\n'}

/* ---------- API ---------- */
let NODE='';
// эти запросы относятся к самой панели, а не к выбранному узлу каскада
const LOCAL={'cascade/nodes':1,'admin/me':1,'admin/sessions':1,'admin/sessions/kill':1,'admin/blocked':1,'admin/unblock':1};
function apiURL(path){const base=path.split('?')[0];return '/admin/api/'+path+((NODE&&!LOCAL[base])?(path.indexOf('?')<0?'?':'&')+'node='+encodeURIComponent(NODE):'')}
async function api(path,body){const r=await fetch(apiURL(path),body?{method:'POST',headers:H,body:JSON.stringify(body)}:{headers:H});if(r.status===404&&path==='state'&&!NODE){location.reload();throw new Error('session')}let j={};try{j=await r.json()}catch(e){}if(!r.ok){const er=new Error(j.error||t('errN',r.status));er.status=r.status;throw er}return j}
async function opt(path){try{return await api(path)}catch(e){return false}}

/* ---------- данные ---------- */
let S=null,NODES=[],nodeDown={},nodeErr='',EV=null,TR=null,ME=null,ASESS=null,BLK=null,PROTO=null,bootErr='',tick=0;
const prevT={},RATE={},hist={};
const PG={overview:'gMon',sessions:'gMon',audit:'gMon',clients:'gAcc',links:'gAcc',cascade:'gNet',protocols:'gNet',cover:'gSrv',settings:'gSrv',security:'gSrv'};
const U={page:'overview',nodeOpen:false,menu:false,q:'',filter:'all',sel:{},cpage:0,drawer:null,dtab:'overview',dr:null,dd:null,links:{},from:null,to:null,auto:true,kind:'all',set:{},pw:{cur:'',n1:'',n2:''},cover:{},modal:null,opt:false,applying:false,switching:false,top:false};
function latMap(){const m={};for(const x of (S&&S.latency||[]))m[x.name]=x;return m}
function pushHist(){const lat=latMap();for(const u of S.upstreams||[]){const a=hist[u.name]||(hist[u.name]=[]);const x=lat[u.name];if(x)a.push(x.ok?x.rtt_ms:-1);while(a.length>16)a.shift()}}
function calcRates(st){const now=Date.now();for(const c of st.clients||[]){const p=prevT[c.name];if(p&&now-p.t>500){const dt=(now-p.t)/1000;RATE[c.name]={down:Math.max(0,(c.tx_bytes-p.tx)*8/dt),up:Math.max(0,(c.rx_bytes-p.rx)*8/dt)}}prevT[c.name]={t:now,rx:c.rx_bytes,tx:c.tx_bytes}}}
async function refresh(){const k=NODE;let st;try{st=await api('state')}catch(e){if(k!==NODE)return;if(NODE){nodeDown[NODE]=true;nodeErr=e.message;U.switching=false}else if(!S)bootErr=e.message;render();return}
if(k!==NODE)return;delete nodeDown[k];nodeErr='';bootErr='';calcRates(st);S=st;U.switching=false;pushHist();
await loadExtras();if(k!==NODE)return;tick++;render();if($('builder').classList.contains('show'))renderBuilder()}
async function loadExtras(){const P=U.page,j=[],again=tick%12===0;// недоступные (старый сервер) — повторяем раз в минуту
if(EV!==false||again)j.push(opt('events?limit=200').then(v=>{EV=v}));
if(BLK!==false||again)j.push(opt('admin/blocked').then(v=>{BLK=v}));
if(TR===null||tick%12===0)j.push(opt('stats/traffic').then(v=>{TR=v}));
if(ME===null)j.push(opt('admin/me').then(v=>{ME=v}));
if(ASESS===null||(P==='security'&&(ASESS!==false||again)))j.push(opt('admin/sessions').then(v=>{ASESS=v}));
if(PROTO===null||((P==='protocols'||P==='overview'||P==='settings')&&(PROTO!==false?tick%2===0:again)))j.push(opt('admin/protocols').then(v=>{PROTO=v}));
if(tick%12===11)j.push(opt('cascade/nodes').then(v=>{if(v)NODES=v.nodes||[]}));
await Promise.all(j)}
function nodeKey(n){return (n.route||[]).join('>')}
function nodeName(){if(!NODE)return t('thisServer');const n=NODES.find(x=>nodeKey(x)===NODE);return n&&n.name||NODE.split('>').pop()}
function nodeBad(n){const k=nodeKey(n);if(nodeDown[k])return true;if(!NODE&&n.route.length===1&&S){const x=latMap()[n.route[0]];if(x&&!x.ok)return true}return false}
function pickNode(k){U.nodeOpen=false;U.menu=false;if(k===NODE){render();return}NODE=k;EV=null;TR=null;PROTO=null;U.switching=true;U.links={};U.set={};U.cover={};U.drawer=null;U.sel={};U.modal=null;U.from=null;U.to=null;for(const x in prevT)delete prevT[x];for(const x in RATE)delete RATE[x];for(const x in hist)delete hist[x];$('builder').classList.remove('show');render();refresh()}
function srvAddr(){const s=S.server;return s.domain||s.public_ipv4||(s.public_ipv6?'['+s.public_ipv6+']':'—')}
function upInfo(name){return (S.upstreams||[]).find(u=>u.name===name)}
function upAddr(u){return u?(u.domain||u.ipv6||u.ipv4||'—'):'—'}
function isGw(c){return (c.subnets||[]).length>0}
function allSessions(){const o=[];S.clients.forEach(c=>(c.sessions||[]).forEach(x=>o.push({c,x})));return o}
function tunnelNet(){const c=S.clients.find(x=>x.address4);return c?c.address4.replace(/\.\d+$/,'.0/24'):'10.66.0.0/24'}

/* ---------- навигация ---------- */
function go(p){if(!PG[p])return;U.page=p;U.nodeOpen=false;U.menu=false;U.dd=null;U.top=true;try{history.replaceState(null,'','#'+p)}catch(e){}render();
if(p==='security')opt('admin/sessions').then(v=>{ASESS=v;if(U.page===p)render()});
if(p==='protocols')opt('admin/protocols').then(v=>{PROTO=v;if(U.page===p)render()})}

/* ---------- компоненты ---------- */
function chip(label,on,fn,cls,extra){return h('button',{type:'button',class:'chip'+(cls?' '+cls:'')+(on?' on':''),'aria-pressed':on?'true':'false',onclick:fn},label,extra?[' ',extra]:null)}
function cbox(state,fn,label,dis){const on=state===true||state==='mixed';return h('button',{type:'button',role:'checkbox','aria-checked':state===true?'true':state==='mixed'?'mixed':'false','aria-label':label,class:'cb'+(on?' on':''),disabled:dis,onclick:fn},state===true?ico('check'):state==='mixed'?h('i'):null)}
function sw(on,small){return h('span',{class:'sw'+(small?' s':'')+(on?' on':''),'aria-hidden':'true'},h('i'))}
function swRow(on,title,sub,fn){return h('button',{type:'button',role:'switch','aria-checked':on?'true':'false',class:'swrow',onclick:fn},h('span',{class:'tt grow'},h('b',null,title),sub?h('small',null,sub):null),sw(on))}
function radio(on,title,sub,fn){return h('button',{type:'button',role:'radio','aria-checked':on?'true':'false',class:'rad'+(on?' on':''),onclick:fn},h('span',{class:'rc'},h('i')),h('span',{class:'tt'},h('b',null,title),sub?h('small',null,sub):null))}
function roleRadios(v,set){return h('div',{class:'col',role:'radiogroup','aria-label':t('roleLabel')},radio(v==='user',t('roleUser'),t('roleUserSub'),()=>set('user')),radio(v==='admin',t('roleAdmin'),t('roleAdminSub'),()=>set('admin')))}
function fld(label,ctl){return h('label',{class:'fld'},label,ctl)}
function inp(f,val,set,a){a=a||{};const p={class:'inp'+(a.cls?' '+a.cls:''),'data-f':f,value:val,oninput:e=>{set(e.target.value);if(a.live)render()}};for(const k in a)if(k!=='cls'&&k!=='live'&&k!=='area')p[k]=a[k];return h(a.area?'textarea':'input',p)}
function dd(key,label,value,opts,pick,dis){const open=U.dd===key;const cur=opts.find(o=>o.v===value)||{label:'—'};const id='dd-'+key;
return h('div',{class:'ddw'+(open?' open':'')},label?h('span',{class:'flab',id},label):null,
h('button',{type:'button',class:'ddb'+(open?' open':''),'aria-haspopup':'listbox','aria-expanded':open?'true':'false','aria-labelledby':label?id:null,disabled:dis,onclick:e=>{e.stopPropagation();U.dd=open?null:key;U.nodeOpen=false;render()}},cur.dot?dot(cur.dot):null,h('span',{class:'grow ell'},cur.label),ico('chev')),
open?h('div',{class:'ddl',role:'listbox','aria-labelledby':label?id:null},opts.map(o=>h('button',{type:'button',role:'option','aria-selected':o.v===value?'true':'false',class:'ddo'+(o.v===value?' on':''),onclick:e=>{e.stopPropagation();U.dd=null;pick(o.v)}},o.dot?dot(o.dot):null,h('span',{class:'grow'},o.label),o.sub?h('span',{class:'num muted',style:'font-size:11px'},o.sub):null,o.v===value?ico('check14'):null))):null)}
function cardHead(title,right){return h('div',{class:'chd'},h('h2',null,title),right)}
function kindInk(k){return {login:'var(--warn)',clients:'var(--hi)',links:'var(--accInk)',cascade:'var(--blue)',settings:'var(--hi)',system:'var(--ink2)'}[k]||'var(--ink2)'}
function kindLabel(k){const s=t('k_'+k);return s==='k_'+k?k:s}

/* ---------- модальные окна ---------- */
function ask(m){U.modal=Object.assign({err:'',busy:false},m);U.dd=null;render();setTimeout(()=>{const f=document.querySelector('.modal [data-f]')||document.querySelector('.modal .mbtns .b:last-child');if(f)f.focus()},0)}
function confirmAsk(title,text,okLabel,run,danger,need){ask({title,text,okLabel,run,danger,field:need!==undefined?'':undefined,ph:need,need})}
function closeModal(){U.modal=null;U.dd=null;render()}
function modalOk(){const M=U.modal;if(!M||M.busy)return;if(M.need!==undefined&&(M.field||'').trim()!==M.need){M.err=t('needMatch',M.need);render();return}
let r;try{r=M.run&&M.run(M)}catch(e){M.err=e.message;render();return}
if(r&&r.then){M.busy=true;M.err='';render();r.then(()=>{if(U.modal===M)U.modal=null;render()},e=>{M.busy=false;M.err=e.message;render()})}else{if(U.modal===M)U.modal=null;render()}}
function showLink(title,r,warn){U.modal={kind:'link',title,warn,link:r.link||r.text||'',qr:r.qr||'',via:r.via||'',file:r.file||((r.name||'veil')+'.txt'),copied:false};U.dd=null;render()}
function modal(){const M=U.modal;
if(M.kind==='link')return h('div',{class:'scrim mid'},h('div',{class:'modal wide',role:'dialog','aria-modal':'true','aria-labelledby':'m-t'},
h('span',{class:'mt',id:'m-t'},M.title),h('div',{class:'dng'},M.warn),M.qr?h('img',{class:'qr',src:M.qr,alt:'QR'}):null,M.via?h('p',{class:'note'},t('routeVia',M.via)):null,
h('textarea',{class:'inp mono',readonly:true,'data-f':'lk',value:M.link,style:'height:96px;font-size:12px',spellcheck:'false','aria-label':M.title}),
h('div',{class:'mbtns'},h('button',{type:'button',class:'b xl',onclick:copyLink},M.copied?t('copied'):t('copy')),h('button',{type:'button',class:'b xl',onclick:()=>download(M.file,M.link+'\n')},t('download')),h('button',{type:'button',class:'b xl acc',onclick:closeModal},t('done')))));
return h('div',{class:'scrim mid'},h('div',{class:'modal'+(M.wide?' wide':''),role:'dialog','aria-modal':'true','aria-labelledby':'m-t'},
h('span',{class:'mt',id:'m-t'},M.title),M.text?h('span',{class:'mx'},M.text):null,M.body?M.body(M):null,
M.field!==undefined?h('input',{class:'inp lg','data-f':'mf',value:M.field,placeholder:M.ph||'','aria-label':M.ph||M.title,oninput:e=>{M.field=e.target.value},onkeydown:e=>{if(e.key==='Enter')modalOk()}}):null,
M.err?h('div',{class:'err',role:'alert'},M.err):null,
h('div',{class:'mbtns'},h('button',{type:'button',class:'b xl ghost',onclick:closeModal},t('cancel')),h('button',{type:'button',class:'b xl '+(M.danger?'acc':'wht'),disabled:M.busy,onclick:modalOk},M.busy?t('wait'):M.okLabel))))}
async function copyLink(){const M=U.modal;try{await navigator.clipboard.writeText(M.link);M.copied=true;render();setTimeout(()=>{if(U.modal===M){M.copied=false;render()}},1500)}catch(e){const x=document.querySelector('[data-f=lk]');if(x){x.focus();x.select()}}}

/* ---------- каркас ---------- */
function render(){const app=$('app');const ae=document.activeElement;const fk=ae&&ae.dataset?ae.dataset.f:null;let s0=null,s1=null;if(fk){try{s0=ae.selectionStart;s1=ae.selectionEnd}catch(e){}}
const scr={};document.querySelectorAll('[data-sc]').forEach(x=>{scr[x.dataset.sc]=x.scrollTop});const wy=window.scrollY;
app.replaceChildren();
if(!S){app.append(boot());return}
document.title='Veil · '+t('p_'+U.page);
app.append(side(),mainCol());
if(U.drawer){const d=drawer();if(d)app.append(d)}
if(U.modal)app.append(modal());
document.querySelectorAll('[data-sc]').forEach(x=>{const k=x.dataset.sc;if(U.top&&k==='main')x.scrollTop=0;else if(scr[k])x.scrollTop=scr[k]});
if(U.top)window.scrollTo(0,0);else if(wy)window.scrollTo(0,wy);U.top=false;
if(fk){const n=document.querySelector('[data-f="'+fk+'"]');if(n){n.focus({preventScroll:true});try{if(s0!==null)n.setSelectionRange(s0,s1)}catch(e){}}}}
function boot(){const w=h('div',{class:'boot'});const l=ico('logo');l.firstChild.setAttribute('width','40');l.firstChild.setAttribute('height','40');w.append(l);
if(bootErr){w.append(h('div',null,t('bootErr',bootErr)),h('button',{type:'button',class:'b lg',onclick:()=>{bootErr='';render();refresh()}},t('retry')))}else w.append(h('div',null,t('loading')));return w}
function side(){const nb=(BLK&&BLK.blocked||[]).length;
const navDef=[['gMon',[['overview'],['sessions',allSessions().length],['audit']]],['gAcc',[['clients',S.clients.length],['links']]],['gNet',[['cascade'],['protocols']]],['gSrv',[['cover'],['settings'],['security',nb,true]]]];
const nav=h('nav',{class:'nav','aria-label':t('navL')},navDef.map(g=>[h('div',{class:'ngrp'},t(g[0])),g[1].map(i=>h('button',{type:'button',class:'nitem'+(U.page===i[0]?' cur':''),'aria-current':U.page===i[0]?'page':null,onclick:()=>go(i[0])},h('span',{class:'grow'},t('p_'+i[0])),i[1]?h('span',{class:'badge'+(i[2]?' warn':'')},String(i[1])):null))]));
const u=ME&&ME.user||'admin';const sub=ME?((ME.role_login?t('loginRole'):t('loginPw'))+(ME.ip?' · '+ME.ip:'')):'';
return h('aside',{class:'side'+(U.menu?' open':'')},
h('div',{class:'brand'},ico('logo'),h('div',{class:'bt'},h('b',null,'Veil Admin'),h('small',null,'v'+(S.server.version||'?')+(ME&&ME.mode==='container'?' · veil-admin':''))),
h('button',{type:'button',class:'ib mbtn','aria-label':t('menu'),'aria-expanded':U.menu?'true':'false',onclick:()=>{U.menu=!U.menu;render()}},ico('menu'))),
nodeSw(),nav,
h('div',{class:'sfoot'},h('span',{class:'ava'},u.charAt(0).toUpperCase()),h('span',{class:'lbl2'},h('span',null,u),sub?h('small',null,sub):null),
h('button',{type:'button',class:'ib','aria-label':t('logout'),title:t('logout'),onclick:logout},ico('logout'))))}
function nodeSw(){const many=NODES.length>1;const down=!!(NODE&&nodeDown[NODE]);
return h('div',{class:'nodesw'},h('button',{type:'button',class:'nodebtn','aria-expanded':many?(U.nodeOpen?'true':'false'):null,'aria-haspopup':many?'listbox':null,'aria-label':t('nodeL')+': '+nodeName(),disabled:!many,onclick:e=>{e.stopPropagation();U.nodeOpen=!U.nodeOpen;U.dd=null;render()}},
dot(down?'var(--acc)':U.switching?'var(--ink2)':'var(--ok)'),h('span',{class:'lbl2'},h('small',null,t('nodeL')),h('span',null,nodeName())),many?ico('chev'):null),
U.nodeOpen&&many?h('div',{class:'pop',role:'listbox','aria-label':t('nodesL')},NODES.map(n=>{const k=nodeKey(n);return h('button',{type:'button',role:'option','aria-selected':k===NODE?'true':'false',class:'opt'+(k===NODE?' on':''),onclick:e=>{e.stopPropagation();pickNode(k)}},dot(nodeBad(n)?'var(--acc)':'var(--ok)'),h('span',{class:'lbl2'},h('span',null,n.name||t('thisServer')),h('small',null,n.route.length?t('viaRoute',n.route.join(' → ')):t('thisServerSub'))))})):null)}
function health(){const a=alerts();const bad=a.filter(x=>x.lvl==='bad').length,w=a.length-bad;const ink=bad?'var(--accInk)':w?'var(--warn)':'var(--ok)';
return h('button',{type:'button',class:'health',style:'color:'+ink,onclick:()=>go('overview'),title:bad?t('hBad',bad):w?t('hWarn',w):t('hOk')},dot(ink),h('span',{class:'ht'},bad?t('hBad',bad):w?t('hWarn',w):t('hOk')))}
function mainCol(){const down=!!(NODE&&nodeDown[NODE]);
const hd=h('header',{class:'top'},h('div',{class:'crumb'},h('small',null,nodeName()+' / '+t(PG[U.page])),h('h1',null,t('p_'+U.page))),h('div',{class:'grow'}),
h('label',{class:'search'},ico('search'),h('span',{class:'sr'},t('search')),h('input',{'data-f':'q',value:U.q,placeholder:t('searchPh'),oninput:e=>{U.q=e.target.value;U.cpage=0;if(U.q&&U.page!=='clients'){U.page='clients';U.top=true;try{history.replaceState(null,'','#clients')}catch(x){}}render()}})),
health(),
h('div',{class:'hdbtns'},h('button',{type:'button',class:'ib lg',title:t('langSwitch'),'aria-label':t('langSwitch'),onclick:switchLang},lang==='ru'?'EN':'RU'),
h('button',{type:'button',class:'ib lg',title:theme==='dark'?t('themeLight'):t('themeDark'),'aria-label':theme==='dark'?t('themeLight'):t('themeDark'),onclick:switchTheme},ico(theme==='dark'?'sun':'moon'))),
h('button',{type:'button',class:'b lg acc',onclick:()=>openNewClient(),disabled:down||U.switching},t('addClient')));
let note=null;
if(down)note=h('div',{class:'notice dng'},h('div',null,h('b',null,t('downTitle',nodeName())),t('downText')+(nodeErr?' ('+nodeErr+')':'')),h('button',{type:'button',class:'b lg',onclick:async()=>{await refresh();if(NODE&&nodeDown[NODE])flash(t('stillDown'),true)}},t('retry')));
else if(U.switching)note=h('div',{class:'notice box'},h('span',{class:'spin'}),h('div',null,t('switching',nodeName())));
else if(NODE)note=h('div',{class:'notice box',style:'background:var(--card)'},t('nodeNote',nodeName()));
const m=h('main',{'data-sc':'main',class:down||U.switching?'dim':null},PAGES[U.page]());
return h('div',{class:'mainc'},hd,note,m,dirtyBar())}
function dirtyBar(){let d=null;
if(U.page==='links'){const n=linksDirty();if(n)d={text:t('dirtyLinks',n),discard:()=>{U.links={};render()},apply:applyLinks}}
if(U.page==='settings'){const n=setDirtyN();if(n)d={text:t('dirtySet',n)+(setVal().front!==setBase().front&&setVal().front?' '+t('dirtyFront'):''),discard:()=>{U.set={};render()},apply:applySettings}}
if(!d)return null;
return h('div',{class:'dirty',role:'region','aria-label':t('unsaved')},dot('var(--acc)'),h('span',{class:'dtext'},d.text),h('button',{type:'button',class:'b lg ghost',onclick:d.discard},t('discard')),h('button',{type:'button',class:'b lg acc',disabled:U.applying,onclick:d.apply},U.applying?t('wait'):t('apply')))}
function switchLang(){lang=lang==='ru'?'en':'ru';store('veil-lang',lang);applyStatic();render();if($('builder').classList.contains('show'))renderBuilder()}
function switchTheme(){theme=theme==='dark'?'light':'dark';store('veil-theme',theme);document.documentElement.dataset.theme=theme;render()}
function applyStatic(){document.documentElement.lang=lang;document.documentElement.dataset.theme=theme;document.querySelectorAll('[data-i]').forEach(e=>{e.textContent=t(e.dataset.i)});$('b-close').setAttribute('aria-label',t('close'))}
async function logout(){try{await fetch('/admin/logout',{method:'POST',headers:H,body:'{}'})}catch(e){}location.href='/admin'}

/* ---------- Обзор ---------- */
function alerts(){if(!S)return [];const a=[];
S.clients.filter(isGw).filter(c=>c.disabled||!c.online).forEach(c=>a.push({lvl:'bad',title:t('aGwOff',c.name),sub:c.subnets.join(', ')+' · '+(c.disabled?t('aGwBlocked'):t('aLast',ago(c.last_handshake))),go:'links'}));
(BLK&&BLK.blocked||[]).forEach(b=>a.push({lvl:'bad',title:t('aBlocked',b.ip,b.fails),sub:t('aUntil',fdt(b.until,true)),go:'security'}));
const s=S.server;if(s.cert&&s.cert.state==='error')a.push({lvl:'bad',title:t('aCert'),sub:s.cert.message||'',go:'settings'});
(S.latency||[]).filter(x=>!x.ok).forEach(x=>a.push({lvl:'bad',title:t('aUp',x.name),sub:x.error||t('noLink'),go:'cascade'}));
if(S.site_repo&&S.site_repo.enabled&&S.site_repo.error)a.push({lvl:'warn',title:t('aRepo'),sub:S.site_repo.error,go:'cover'});
(PROTO&&PROTO.containers||[]).filter(c=>c.state&&c.state!=='running').forEach(c=>a.push({lvl:'warn',title:t('aCtr',c.name,c.state),sub:c.status||'',go:'protocols'}));
return a}
function pgOverview(){const s=S.server,cl=S.clients,ss=allSessions();const gws=cl.filter(isGw),gwOff=gws.filter(c=>c.disabled||!c.online);
const h3=ss.filter(x=>/3/.test(x.x.transport||'')).length;
let tr=null;if(TR&&TR.hours&&TR.hours.length){tr={rx:0,tx:0};TR.hours.forEach(x=>{tr.rx+=+x.rx||0;tr.tx+=+x.tx||0})}
const kp=[[t('kClients'),String(cl.length),t('kClientsSub',cl.filter(c=>c.role==='admin').length,cl.filter(c=>c.disabled).length),'','clients'],
[t('kOnline'),String(s.online),'HTTP/3: '+h3+' · HTTP/2: '+(ss.length-h3),'','sessions'],
tr?[t('kTraffic'),bytes(tr.rx+tr.tx),t('rxS')+' '+bytes(tr.rx)+' · '+t('txS')+' '+bytes(tr.tx),'',null]:[t('kTrafficAll'),bytes(s.rx_bytes+s.tx_bytes),t('kSinceStart'),'',null],
[t('kSites'),String(gws.length),gwOff.length?t('kGwOff',gwOff.length):(gws.length?t('kGwOk'):t('kGwNone')),gwOff.length?'var(--accInk)':gws.length?'var(--ok)':'','links'],
[t('kUptime'),dur(s.started),t('kVer',s.version),'','settings']];
const kpis=h('div',{class:'g5'},kp.map(k=>h(k[4]?'button':'div',k[4]?{type:'button',class:'kpi',onclick:()=>go(k[4])}:{class:'kpi'},h('span',null,k[0]),h('b',{title:k[1]},k[1]),h('small',{style:k[3]?'color:'+k[3]:null,title:k[2]},k[2]))));
const al=alerts();
const chartCard=h('section',{class:'card','aria-label':t('trTitle')},h('div',{class:'row'},h('h2',{class:'grow'},t('trTitle')),tr?[h('span',{class:'lg'},h('i'),t('rxS')),h('span',{class:'lg'},h('i',{class:'tx'}),t('txS'))]:null),chart());
const attn=h('section',{class:'card','aria-label':t('attn')},h('h2',null,t('attn')),al.length?al.slice(0,6).map(x=>h('button',{type:'button',class:'alert',onclick:()=>go(x.go)},dot(x.lvl==='bad'?'var(--acc)':'var(--warn)'),h('span',{class:'tt'},h('b',null,x.title),x.sub?h('small',null,x.sub):null))):h('div',{class:'box'},t('allOk')));
const svc=h('section',{class:'card','aria-label':t('svcTitle')},h('h2',null,t('svcTitle')),h('div',null,services()));
const evc=h('section',{class:'card','aria-label':t('evTitle')},h('div',{class:'row'},h('h2',{class:'grow'},t('evTitle')),EV?h('button',{type:'button',class:'lnkb',onclick:()=>go('audit')},t('evAll')):null),
h('div',null,EV===null?h('div',{class:'box'},t('loading')):!EV?h('div',{class:'box'},t('evNA')):!(EV.events||[]).length?h('div',{class:'box'},t('evNone')):EV.events.slice(0,5).map(e=>h('div',{class:'lrow'},h('span',{class:'tm'},fdt(e.time,true)),h('span',{class:'kd',style:'color:'+kindInk(e.kind)},kindLabel(e.kind)),h('span',{class:'grow ell',title:e.text},e.text)))));
return [kpis,h('div',{class:'g21'},chartCard,attn),h('div',{class:'g2'},svc,evc)]}
function chart(){if(TR===null)return h('div',{class:'box'},t('loading'));if(!TR||!TR.hours||!TR.hours.length)return h('div',{class:'box'},t('trNA'));
const hs=TR.hours.slice(-24);const v=x=>(+x.rx||0)+(+x.tx||0);const mx=Math.max.apply(null,hs.map(v).concat([1]));let pk=hs[0];hs.forEach(x=>{if(v(x)>v(pk))pk=x});
const tm=x=>{const d=okDate(x.t);return d?hm(d):''};
const bars=h('div',{class:'chart',role:'img','aria-label':t('trPeak',tm(pk),bytes(v(pk)))},hs.map(x=>h('div',{title:tm(x)+' · '+t('rxS')+' '+bytes(x.rx)+' · '+t('txS')+' '+bytes(x.tx)},h('i',{style:'height:'+(100*(+x.rx||0)/mx).toFixed(1)+'%'}),h('i',{class:'tx',style:'height:'+(100*(+x.tx||0)/mx).toFixed(1)+'%'}))));
const idx=[0,6,12,18,hs.length-1].filter((n,i,a)=>n<hs.length&&a.indexOf(n)===i);
return [bars,h('div',{class:'axis'},idx.map(i=>h('span',null,tm(hs[i]))))]}
function ctrs(){return PROTO&&PROTO.containers||[]}
function ctrDot(c){return !c?'var(--off)':c.state==='running'?'var(--ok)':(c.state==='restarting'||c.state==='created')?'var(--warn)':'var(--acc)'}
function services(){const rows=[];if(ctrs().length)ctrs().forEach(c=>rows.push([ctrDot(c),c.name,c.status||c.state||'',c.ports||'']));else rows.push(['var(--ok)','veil',t('running'),S.server.port+'/tcp+udp']);
const out=rows.map(r=>h('div',{class:'lrow'},dot(r[0]),h('span',{class:'lname ell'},r[1]),h('span',{class:'muted grow ell'},r[2]),h('span',{class:'num muted sm ell',style:'max-width:40%'},r[3])));
if(PROTO===false)out.push(h('p',{class:'note',style:'margin-top:8px'},t('svcNA')));return out}

/* ---------- Клиенты ---------- */
const FILTERS=[['all',()=>true],['online',c=>c.online&&!c.disabled],['gw',isGw],['admin',c=>c.role==='admin'],['blocked',c=>c.disabled]];
function clientList(){const f=(FILTERS.find(x=>x[0]===U.filter)||FILTERS[0])[1];const q=U.q.trim().toLowerCase();
return S.clients.filter(f).filter(c=>!q||[c.name,c.note,c.address4,c.address6,(c.subnets||[]).join(' '),(c.route||[]).join(' '),(c.sessions||[]).map(s=>s.remote).join(' ')].join(' ').toLowerCase().indexOf(q)>=0)}
function roleOf(c){if(c.relay_of)return [t('roleTransit'),'var(--blue)','var(--card2)'];return c.role==='admin'?[t('roleAdminS'),'var(--accInk)','var(--dngBg)']:c.role==='user'?[t('roleUserS'),'var(--hi)','var(--card2)']:[t('roleNone'),'var(--ink2)','transparent']}
function stOf(c){if(c.disabled)return [t('stBlocked'),'var(--accInk)'];if(c.online){const tr=c.sessions&&c.sessions[0]&&c.sessions[0].transport;return [t('stOnline')+(tr?' · '+tr:''),'var(--ok)']}return [t('stOffline'),'var(--ink2)']}
function routeOf(c){if(c.relay_of)return t('rTransit',c.relay_of);if(c.auto)return t('rAuto',c.auto.exit)+(c.auto.cost_ms>0?' · '+ms1(c.auto.cost_ms):c.auto.cost_ms<0?' · '+t('unreachable'):'');if(c.route&&c.route.length)return t('rVia',c.route.join(' → '));return t('rDirect')}
function selNames(){return Object.keys(U.sel).filter(k=>U.sel[k]&&S.clients.some(c=>c.name===k&&!c.relay_of))}
async function bulkRun(names,fn,okKey){const errs=[];for(const n of names){try{await fn(n)}catch(e){errs.push(n+': '+e.message)}}U.sel={};const ok=names.length-errs.length;
if(errs.length)flash((ok?t(okKey,ok)+'\n':'')+t('bulkErr',errs.length,errs.slice(0,3).join('; ')),true,6000);else flash(t(okKey,ok));await refresh()}
function pgClients(){const list=clientList();const per=25;const pages=Math.max(1,Math.ceil(list.length/per));if(U.cpage>=pages)U.cpage=pages-1;const shown=list.slice(U.cpage*per,U.cpage*per+per);
const selectable=list.filter(c=>!c.relay_of);const nSel=selectable.filter(c=>U.sel[c.name]).length;const all=nSel>0&&nSel===selectable.length;const names=selNames();
const bar=h('div',{class:'row'},FILTERS.map(f=>chip(t('f_'+f[0]),U.filter===f[0],()=>{U.filter=f[0];U.sel={};U.cpage=0;render()},'',h('span',{class:'num',style:'opacity:.8'},String(S.clients.filter(f[1]).length)))),h('div',{class:'grow'}),h('button',{type:'button',class:'b flat',onclick:exportClients},t('exportCsv')));
const bulk=names.length?h('div',{class:'bulk'},h('span',null,t('selN',names.length)),
h('button',{type:'button',class:'b',onclick:()=>confirmAsk(t('bulkBlockT',names.length),t('bulkBlockX'),t('block'),()=>bulkRun(names,n=>api('clients/update',{name:n,disabled:true}),'bulkBlocked'),true)},t('block')),
h('button',{type:'button',class:'b',onclick:()=>bulkRun(names,n=>api('clients/update',{name:n,disabled:false}),'bulkUnblocked')},t('unblock')),
h('button',{type:'button',class:'b red',onclick:()=>confirmAsk(t('bulkDelT',names.length),t('bulkDelX'),t('del'),()=>bulkRun(names,n=>api('clients/delete',{name:n}),'bulkDeleted'),true,String(names.length))},t('del')),
h('button',{type:'button',class:'ib','aria-label':t('clearSel'),onclick:()=>{U.sel={};render()}},'✕')):null;
const head=h('tr',null,h('th',{style:'width:44px'},cbox(all?true:nSel?'mixed':false,()=>{const s={};if(!all)selectable.forEach(c=>{s[c.name]=true});U.sel=s;render()},t('selAll'),!selectable.length)),
h('th',null,t('thClient')),h('th',null,t('thRole')),h('th',null,t('thAddr')),h('th',null,t('thRoute')),h('th',null,t('thState')),h('th',{class:'r'},t('thTraffic')),h('th',{style:'width:110px'}));
const rows=shown.map(c=>{const [rl,ri,rb]=roleOf(c);const [st,si]=stOf(c);const sel=!!U.sel[c.name]&&!c.relay_of;
return h('tr',{class:sel?'sel':null},h('td',null,cbox(sel,()=>{U.sel[c.name]=!sel;render()},t('selOne',c.name),!!c.relay_of)),
h('td',null,h('button',{type:'button',class:'cbtn',onclick:()=>openClient(c.name)},h('b',null,c.name,isGw(c)?h('span',{class:'gw'},t('gwTag')):null),h('small',{class:'ell'},c.note||'—'))),
h('td',null,h('span',{class:'pill',style:'color:'+ri+';background:'+rb},rl)),h('td',{class:'addr'},c.address4||'—'),h('td',{class:'muted sm',style:'font-weight:500;max-width:220px'},routeOf(c)),
h('td',null,h('span',{class:'st',style:'color:'+si},dot(si),st)),h('td',{class:'r sm',style:'white-space:nowrap'},bytes(c.tx_bytes)+' / '+bytes(c.rx_bytes)),
h('td',{class:'r'},h('button',{type:'button',class:'b sm',onclick:()=>openClient(c.name)},t('open'))))});
const tbl=h('section',{class:'card p0','aria-label':t('p_clients')},h('div',{class:'tw'},h('table',{class:'t',style:'min-width:820px'},h('thead',null,head),h('tbody',null,rows))),
!shown.length?h('div',{class:'empty'},S.clients.length?t('noRows'):t('noClients')):null,
h('div',{class:'tfoot'},h('span',{class:'grow'},t('shownOf',shown.length,list.length)),pages>1?[h('span',null,t('pageOf',U.cpage+1,pages)),h('button',{type:'button',class:'b sm',disabled:U.cpage===0,onclick:()=>{U.cpage--;U.top=true;render()}},t('prev')),h('button',{type:'button',class:'b sm',disabled:U.cpage>=pages-1,onclick:()=>{U.cpage++;U.top=true;render()}},t('next'))]:h('span',null,t('perPage'))));
return [bar,bulk,tbl]}
function exportClients(){const name='veil-clients-'+today()+'.csv';const rows=[['name','note','role','address4','address6','route','online','disabled','rx_bytes','tx_bytes','subnets','created','last_handshake']];
clientList().forEach(c=>rows.push([c.name,c.note||'',c.relay_of?'transit':c.role||'',c.address4,c.address6,routeOf(c),c.online,c.disabled,c.rx_bytes,c.tx_bytes,(c.subnets||[]).join(' '),c.created||'',c.last_handshake||'']));download(name,csv(rows),'text/csv;charset=utf-8');flash(t('csvSaved',name))}

/* ---------- карточка клиента ---------- */
function openClient(name){const c=S.clients.find(x=>x.name===name);if(!c)return;U.drawer=name;U.dtab='overview';U.dr={name:c.name,note:c.note||'',role:c.role||'',nets:(c.subnets||[]).join('\n'),err:'',netsErr:'',busy:false};U.dd=null;render()}
function closeDrawer(){U.drawer=null;U.dr=null;render()}
function validNets(text){const tn=tunnelNet(),p=tn.split('.');
for(const v of text.split(/[\s,;]+/).filter(Boolean)){const m=v.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})(?:\/(\d{1,2}))?$/);
if(!m&&!/^[0-9a-fA-F:]+:[0-9a-fA-F:]*(\/\d{1,3})?$/.test(v))return t('nets1',v);
if(m&&m.slice(1,5).some(x=>+x>255))return t('nets2',v);
if(m&&m[5]!==undefined&&+m[5]<8)return t('nets3',v);
if(m&&m[1]===p[0]&&m[2]===p[1]&&m[3]===p[2])return t('nets4',v,tn)}
return ''}
async function saveClient(){const c=S.clients.find(x=>x.name===U.drawer),d=U.dr;if(!c||!d||c.relay_of||d.busy)return;
const nets=d.nets.split(/[\s,;]+/).filter(Boolean);const netsCh=nets.join(',')!==(c.subnets||[]).join(',');
if(netsCh){const e=validNets(d.nets);if(e){d.netsErr=e;U.dtab='nets';render();return}}
const body={name:c.name};let ch=false;const nn=d.name.trim();if(nn&&nn!==c.name){body.new_name=nn;ch=true}if(d.note!==(c.note||'')){body.note=d.note;ch=true}if(d.role&&d.role!==(c.role||'')){body.role=d.role;ch=true}
if(!ch&&!netsCh){closeDrawer();return}
d.busy=true;d.err='';d.netsErr='';render();let step='upd';
try{if(ch)await api('clients/update',body);const nm=body.new_name||c.name;U.drawer=nm;step='nets';if(netsCh)await api('clients/subnets',{name:nm,subnets:nets});flash(t('clientSaved'));U.drawer=null;U.dr=null;await refresh()}
catch(e){d.busy=false;if(step==='nets'){d.netsErr=e.message;U.dtab='nets'}else d.err=e.message;await refresh()}}
function drawer(){const c=S.clients.find(x=>x.name===U.drawer);if(!c){U.drawer=null;U.dr=null;return null}const d=U.dr;const tr=!!c.relay_of;
const tabs=[['overview','tOverview'],['config','tConfig'],['nets','tNets'],['access','tAccess'],['history','tHistory']].filter(x=>!tr||x[0]==='overview'||x[0]==='history');
if(!tabs.some(x=>x[0]===U.dtab))U.dtab='overview';
const [st,si]=stOf(c);const rl=roleOf(c)[0];
const sub=(c.role==='admin'?t('roleAdmin'):c.role==='user'?t('roleUser'):rl)+' · '+(c.address4||'—')+(isGw(c)?' · '+t('gwSite'):'');
let body=[];
if(U.dtab==='overview'){body=[h('div',{class:'kv'},h('span',null,t('dState')),h('span',{style:'font-weight:600;color:'+si},st),h('span',null,t('dAddrs')),h('span',{class:'num'},[c.address4,c.address6].filter(Boolean).join(', ')||'—'),
h('span',null,t('dRoute')),h('span',null,routeOf(c)),h('span',null,t('dCreated')),h('span',null,fdate(c.created)),h('span',null,t('dHs')),h('span',null,ago(c.last_handshake)),
h('span',null,t('dTraffic')),h('span',{class:'num'},t('dTrafficV',bytes(c.tx_bytes),bytes(c.rx_bytes))),
(c.sessions||[]).length?[h('span',null,t('dSess')),h('span',{class:'num'},c.sessions.map(x=>h('div',null,x.transport+' · '+x.remote)))]:null),
tr?h('div',{class:'box'},t('transitNote',c.relay_of)):[fld(t('name'),inp('d-name',d.name,v=>{d.name=v},{maxlength:40})),fld(t('note'),inp('d-note',d.note,v=>{d.note=v},{maxlength:200}))]]}
else if(U.dtab==='config')body=[h('div',{class:'box',style:'padding:14px'},t('cfgBox')),h('div',null,h('button',{type:'button',class:'b lg acc',onclick:()=>confirmAsk(t('reT',c.name),t('reX'),t('reissue'),async()=>{const r=await api('clients/reissue',{name:c.name});showLink(t('newCfgTitle',c.name),r,t('newCfgWarn'));refresh()},true)},t('reissue')))];
else if(U.dtab==='nets'){const ve=d.netsErr||validNets(d.nets);body=[h('p',{class:'note'},t('netsX')+' ',h('code',{style:'color:var(--hi)'},'sudo veil connect -gateway')),
inp('d-nets',d.nets,v=>{d.nets=v;d.netsErr=''},{area:1,live:1,cls:ve?'bad':'',spellcheck:'false',placeholder:'192.168.1.0/24','aria-label':t('tNets')}),
h('span',{class:ve?'err':'note',style:'font-weight:600'},ve||(d.nets.trim()?t('netsWill'):t('netsEmpty')))]}
else if(U.dtab==='access')body=[h('span',{class:'flab'},t('roleLabel')),roleRadios(d.role,v=>{d.role=v;render()}),d.role==='admin'&&c.role!=='admin'?h('div',{class:'dng'},t('newAdminWarn')):null,
swRow(c.disabled,t('blockedL'),t('blockedSub'),async()=>{try{await api('clients/update',{name:c.name,disabled:!c.disabled});flash(c.disabled?t('blockedOff'):t('blockedOn'));await refresh()}catch(e){flash(e.message,true)}})];
else{const ev=EV&&EV.events?EV.events.filter(e=>(e.text||'').indexOf(c.name)>=0):null;
body=[ev===null?h('div',{class:'box'},EV===null?t('loading'):t('evNA')):null,ev?ev.map(e=>h('div',{class:'hist'},h('span',{class:'num'},fdt(e.time)),h('span',null,e.text))):null,
okDate(c.created)?h('div',{class:'hist'},h('span',{class:'num'},fdt(c.created)),h('span',null,t('histCreated'))):null,ev&&!ev.length&&!okDate(c.created)?h('div',{class:'box'},t('histNone')):null]}
return h('div',{class:'scrim',onclick:e=>{if(e.target===e.currentTarget)closeDrawer()}},h('aside',{class:'drawer',role:'dialog','aria-modal':'true','aria-label':t('clientL',c.name)},
h('div',{class:'dh'},h('div',{class:'tt grow'},h('b',null,c.name),h('small',null,sub)),h('button',{type:'button',class:'ib x','aria-label':t('close'),onclick:closeDrawer},'✕')),
h('div',{class:'tabs',role:'tablist'},tabs.map(x=>h('button',{type:'button',role:'tab','aria-selected':U.dtab===x[0]?'true':'false',class:'tab'+(U.dtab===x[0]?' on':''),onclick:()=>{U.dtab=x[0];render()}},t(x[1])))),
h('div',{class:'dbody','data-sc':'drawer'},body,d.err?h('div',{class:'err',role:'alert'},d.err):null),
h('div',{class:'dfoot'},tr?null:h('button',{type:'button',class:'b lg ghost red',onclick:()=>confirmAsk(t('delT',c.name),t('delX',isGw(c)?t('delNets',c.subnets.join(', ')):''),t('del'),async()=>{await api('clients/delete',{name:c.name});U.drawer=null;U.dr=null;flash(t('deleted'));refresh()},true,c.name)},t('delClient')),
h('div',{class:'grow'}),h('button',{type:'button',class:'b lg',onclick:closeDrawer},t('close')),tr?null:h('button',{type:'button',class:'b lg acc',disabled:d.busy,onclick:saveClient},d.busy?t('wait'):t('save')))))}

/* ---------- новый клиент ---------- */
function openNewClient(pre){pre=pre||{};const ups=S.upstreams||[];
const M={title:t('newClient'),okLabel:t('create'),wide:true,nc:{name:'',note:'',role:'user',mode:pre.mode==='auto'&&ups.length?'auto':'manual',route:(pre.route||[]).slice(),exit:pre.exit||(ups[0]?ups[0].name:''),hops:pre.hops!==undefined?String(pre.hops):'-1',opts:null}};
M.body=ncBody;M.run=async()=>{const n=M.nc;const body=n.mode==='auto'?{name:n.name,note:n.note,role:n.role,auto:true,exit:n.exit,max_hops:parseInt(n.hops,10)}:{name:n.name,note:n.note,role:n.role,route:n.route};
const r=await api('clients',body);showLink(t('createdTitle',r.name),r,t('createdWarn'));refresh()};
ask(M);ncExplore(M)}
function ncExplore(M){const n=M.nc;n.opts=null;n.optErr='';if(!(S.upstreams||[]).length){n.opts=[];return}
api('explore',{route:n.route}).then(r=>{n.opts=(r.upstreams||[]).filter(o=>n.route.indexOf(o)<0)},e=>{n.optErr=e.message;n.opts=[]}).then(()=>{if(U.modal===M)render()})}
function ncBody(M){const n=M.nc;const ups=S.upstreams||[];const out=[fld(t('name'),inp('nc-name',n.name,v=>{n.name=v},{maxlength:40,placeholder:t('namePh')})),fld(t('note'),inp('nc-note',n.note,v=>{n.note=v},{maxlength:200})),
h('span',{class:'flab'},t('roleLabel')),roleRadios(n.role,v=>{n.role=v;render()}),n.role==='admin'?h('div',{class:'dng'},t('newAdminWarn')):null];
if(!ups.length){out.push(h('p',{class:'note'},t('noCascadeRoute')));return out}
out.push(dd('nc-mode',t('route'),n.mode,[{v:'manual',label:t('modeManualOpt')},{v:'auto',label:t('modeAutoOpt')}],v=>{n.mode=v;render()}));
if(n.mode==='manual'){const hops=h('div',{class:'hops'},n.route.map((x,i)=>h('div',{class:'hop'},h('span',{class:'pill'},(i+1)+'. '+x),i===n.route.length-1?h('button',{type:'button',class:'lnkb',onclick:()=>{n.route.pop();ncExplore(M);render()}},t('undo')):null)));out.push(hops);
if(n.opts===null)out.push(h('p',{class:'note'},t('loading')));
else if(n.optErr)out.push(h('div',{class:'err'},n.optErr));
else if(n.opts.length)out.push(dd('nc-next',t('nextHop'),'',[{v:'',label:n.route.length?t('exitOnLast'):t('exitOnThis')}].concat(n.opts.map(o=>({v:o,label:t('nextThrough',o)}))),v=>{if(v){n.route.push(v);ncExplore(M)}render()}));
else out.push(h('p',{class:'note'},n.route.length?t('noMoreNodes'):t('noCascadeRoute')))}
else{out.push(dd('nc-exit',t('exitNode'),n.exit,ups.map(u=>({v:u.name,label:u.name,sub:u.company})),v=>{n.exit=v;render()}),
dd('nc-hops',t('hopsCount'),n.hops,[{v:'-1',label:t('allAvail')}].concat(ups.map((u,i)=>({v:String(i),label:String(i)}))),v=>{n.hops=v;render()}),h('p',{class:'note'},t('autoHint')))}
return out}

/* ---------- Сессии ---------- */
function pgSessions(){const ss=allSessions();const fam=r=>/^\[|:.*:/.test(r||'')?'IPv6':'IPv4';const h3=ss.filter(x=>/3/.test(x.x.transport||'')).length;
const top=h('div',{class:'row'},h('span',{class:'grow',style:'font-size:13px;font-weight:500;color:var(--ink2)'},t('sessHead',ss.length,h3,ss.length-h3,ss.filter(x=>fam(x.x.remote)==='IPv6').length)),
h('button',{type:'button',role:'switch','aria-checked':U.auto?'true':'false',class:'b flat',onclick:()=>{U.auto=!U.auto;render()}},sw(U.auto,true),t('autoRef')));
const rows=ss.map(o=>{const c=o.c,x=o.x,r=RATE[c.name];return h('tr',null,h('td',{style:'font-weight:600'},c.name),h('td',null,h('span',{class:'pill',style:'background:var(--card2);color:'+(/3/.test(x.transport||'')?'var(--ok)':'var(--hi)')},x.transport||'—')),
h('td',{class:'muted sm'},fam(x.remote)),h('td',{class:'addr'},x.remote||'—'),h('td',{class:'muted sm'},c.relay_of?t('entryRelay',c.relay_of):t('entryDirect')),h('td',{class:'sm'},fdt(x.since,true)),
h('td',{class:'r sm',style:'white-space:nowrap'},r?bits(r.down)+' / '+bits(r.up):'—'),
h('td',{class:'r'},h('button',{type:'button',class:'b sm red',onclick:()=>confirmAsk(t('killT',c.name),t('killX'),t('kill'),async()=>{await api('sessions/kill',{name:c.name});flash(t('killed'));refresh()},true)},t('kill'))))});
return [top,h('section',{class:'card p0','aria-label':t('p_sessions')},h('div',{class:'tw'},h('table',{class:'t c48',style:'min-width:860px'},h('thead',null,h('tr',null,h('th',null,t('thClient')),h('th',null,t('thTransport')),h('th',null,t('thFamily')),h('th',null,t('thRemote')),h('th',null,t('thEntry')),h('th',null,t('thSince')),h('th',{class:'r'},t('thRate')),h('th',{style:'width:120px'}))),h('tbody',null,rows))),ss.length?null:h('div',{class:'empty'},t('noSess'))),
h('p',{class:'note'},t('sessNote'))]}

/* ---------- Журнал ---------- */
function pgAudit(){if(EV===null)return h('div',{class:'box'},t('loading'));if(!EV)return h('div',{class:'box'},t('evNA'));
const kinds=['all','login','clients','links','cascade','settings','system'];const all=EV.events||[];const list=U.kind==='all'?all:all.filter(e=>e.kind===U.kind);
return [h('div',{class:'row'},kinds.map(k=>chip(kindLabel(k),U.kind===k,()=>{U.kind=k;render()})),h('div',{class:'grow'}),h('button',{type:'button',class:'b flat',onclick:()=>{const name='veil-events-'+today()+'.csv';const rows=[['time','kind','actor','ip','text']];list.forEach(e=>rows.push([e.time,e.kind,e.actor||'',e.ip||'',e.text||'']));download(name,csv(rows),'text/csv;charset=utf-8');flash(t('csvSaved',name))}},t('export'))),
h('section',{class:'card p0','aria-label':t('p_audit')},h('div',{class:'tw'},h('table',{class:'t c44',style:'min-width:720px'},h('thead',null,h('tr',null,h('th',{style:'width:120px'},t('evTime')),h('th',{style:'width:110px'},t('evKind')),h('th',{style:'width:170px'},t('evWho')),h('th',null,t('evText')),h('th',{style:'width:130px'},t('evIp')))),
h('tbody',null,list.map(e=>h('tr',null,h('td',{class:'sm'},fdt(e.time)),h('td',null,h('span',{style:'font-size:11px;font-weight:700;color:'+kindInk(e.kind)},kindLabel(e.kind))),h('td',{style:'font-weight:600'},e.actor||'—'),h('td',null,e.text),h('td',{class:'sm'},e.ip||'—')))))),list.length?null:h('div',{class:'empty'},t('evNone')))]}

/* ---------- Связи сетей ---------- */
function linksBase(){const l=S.server.links||{};return {c2c:!!l.client_to_client,c2s:!!l.client_to_site,s2s:!!l.site_to_site}}
function linksVal(){const b=linksBase();for(const k in U.links)b[k]=U.links[k];return b}
function linksDirty(){const b=linksBase(),v=linksVal();return ['c2c','c2s','s2s'].filter(k=>b[k]!==v[k]).length}
async function applyLinks(){const v=linksVal();U.applying=true;render();try{await api('settings/links',{client_to_client:v.c2c,client_to_site:v.c2s,site_to_site:v.s2s});U.links={};flash(t('lkSaved'));await refresh()}catch(e){flash(e.message,true)}finally{U.applying=false;render()}}
function pgLinks(){const b=linksBase(),L=linksVal();const gws=S.clients.filter(isGw);const sites=[];gws.forEach(c=>c.subnets.forEach(n=>sites.push({net:n,c})));
const def=[['c2c','lkC2C','lkC2CH',t('nClients',S.clients.length)],['c2s','lkC2S','lkC2SH',t('nNets',sites.length)],['s2s','lkS2S','lkS2SH',t('nGws',gws.length)]];
const cards=h('div',{class:'g3'},def.map(d=>{const on=L[d[0]],ch=on!==b[d[0]];return h('button',{type:'button',role:'switch','aria-checked':on?'true':'false',class:'lcard'+(ch?' ch':''),onclick:()=>{U.links[d[0]]=!on;if(U.links[d[0]]===b[d[0]])delete U.links[d[0]];render()}},
h('span',{class:'lt'},h('span',{class:'grow'},t(d[1])),sw(on)),h('span',{class:'note'},t(d[2])),h('span',{style:'font-size:12px;font-weight:600;color:'+(ch?'var(--accInk)':'var(--ink2)')},ch?t('lkChanged'):(on?t('lkOn',d[3]):t('lkOff',d[3]))))}));
const stbl=h('section',{class:'card p0','aria-label':t('sitesT')},cardHead(t('sitesT'),h('button',{type:'button',class:'lnkb',onclick:()=>{U.filter='all';go('clients')}},t('assignGw'))),
sites.length?h('div',{class:'tw'},h('table',{class:'t c44',style:'min-width:520px'},h('thead',null,h('tr',null,h('th',null,t('thNet')),h('th',null,t('thGw')),h('th',null,t('thGwOn')),h('th',null,t('thSrvRoute')))),
h('tbody',null,sites.map(s=>{const c=s.c,on=c.online&&!c.disabled;return h('tr',null,h('td',{class:'num'},s.net),h('td',null,h('button',{type:'button',class:'cbtn',onclick:()=>openClient(c.name)},h('b',null,c.name))),h('td',{style:'font-size:12px;font-weight:600;color:'+(on?'var(--ok)':'var(--accInk)')},c.disabled?t('stBlocked'):c.online?t('stOnline'):t('stOffline')),h('td',{class:'muted sm'},c.disabled?t('routeOff'):'veil0 → '+(c.address4||'—')))})))):h('div',{style:'padding:0 16px 16px'},h('div',{class:'box'},t('noSites'))));
// проверка доступа
const tn=tunnelNet();const pal=['var(--ok)','var(--warn)','var(--blue)','var(--hi)'];
const eps=[{v:'c',label:t('epClient'),sub:tn,dot:'var(--hi)',k:'c'},{v:'c2',label:t('epClient2'),sub:tn,dot:'var(--hi)',k:'c'}].concat(sites.map((s,i)=>({v:'s:'+i,label:t('epSite',s.c.name),sub:s.net,dot:pal[i%pal.length],k:'s',c:s.c})),[{v:'net',label:t('epNet'),sub:'0.0.0.0/0',dot:'var(--ink3)',k:'n'}]);
if(!eps.some(e=>e.v===U.from))U.from='c';if(!eps.some(e=>e.v===U.to))U.to=sites.length?'s:0':'c2';
const fe=eps.find(e=>e.v===U.from),te=eps.find(e=>e.v===U.to);let pr;
if(U.from===U.to)pr={v:t('prSame'),why:'',ink:'var(--ink2)'};
else if(fe.k==='n'||te.k==='n')pr={v:t('prPass'),why:t('prNat'),ink:'var(--ok)'};
else{const key=fe.k==='c'&&te.k==='c'?'c2c':fe.k==='s'&&te.k==='s'?'s2s':'c2s';const nm=t({c2c:'lkC2C',c2s:'lkC2S',s2s:'lkS2S'}[key]);const dn=[fe,te].find(e=>e.k==='s'&&(e.c.disabled||!e.c.online));
pr=L[key]?(dn?{v:t('prDown'),why:t('prGwDown',nm,dn.c.name),ink:'var(--warn)'}:{v:t('prPass'),why:t('prRule',nm),ink:'var(--ok)'}):{v:t('prBlock'),why:t('prRuleOff',nm),ink:'var(--accInk)'}}
const probe=h('section',{class:'card','aria-label':t('probeT')},h('h2',null,t('probeT')),h('p',{class:'note'},t('probeX')),
dd('from',t('from'),U.from,eps,v=>{U.from=v;render()}),dd('to',t('to'),U.to,eps,v=>{U.to=v;render()}),
h('div',{class:'verdict',role:'status'},dot(pr.ink),h('span',{class:'tt'},h('b',{style:'color:'+pr.ink},pr.v),pr.why?h('small',null,pr.why):null)));
return [cards,h('div',{class:'g141'},stbl,probe)]}

/* ---------- Каскад ---------- */
function pickClient(){const c=S.clients.filter(c=>!c.relay_of&&c.route&&c.route.length);c.sort((a,b)=>((b.online?1:0)-(a.online?1:0))||(new Date(b.last_handshake||0)-new Date(a.last_handshake||0)));return c[0]||null}
function pgCascade(){const c=pickClient(),route=c?c.route:[];const lat=latMap();
const seq=[{role:t('source'),name:t('clients'),addr:t('onlineOf',S.server.online,S.clients.length)},{role:route.length?t('thisNode'):t('thisNodeExit'),name:nodeName(),addr:srvAddr(),me:true}].concat(route.map((n,i)=>({role:i===route.length-1?t('exit'):t('inter'),name:n,addr:upAddr(upInfo(n))})));
const link=i=>{if(i===1&&route.length){const x=lat[route[0]];if(!x)return {l:t('measuring'),ink:'var(--ink3)'};return x.ok?{l:ms1(x.rtt_ms),ink:x.rtt_ms<100?'var(--ok)':x.rtt_ms<200?'var(--warn)':'var(--accInk)'}:{l:t('noLink'),ink:'var(--accInk)'}}return {l:i===0?'':'—',ink:'var(--ink3)'}};
const chain=h('div',{class:'chain'},seq.map((n,i)=>{const lk=i<seq.length-1?link(i):null;return h('div',{class:'cw'},h('div',{class:'cn'+(n.me?' me':'')},h('small',null,n.role),h('b',{title:n.name},n.name),h('span',{title:n.addr},n.addr)),lk?h('div',{class:'cl',style:'color:'+lk.ink},h('span',null,lk.l),h('i',{style:'background:'+lk.ink})):null)}));
const sch=h('section',{class:'card','aria-label':t('chainT')},h('div',{class:'row'},h('h2',{class:'grow'},t('chainT')),h('button',{type:'button',class:'b',onclick:openBuilder},t('builderOpen')),h('button',{type:'button',class:'b',disabled:U.opt,onclick:optimize},U.opt?t('rebuilding'):t('rebuild'))),
chain,h('p',{class:'note'},(c?t('chainOf',c.name,c.auto?t('modeAuto'):t('modeManual')):t('chainNone'))+' · '+t('chainX')));
const ups=h('section',{class:'card p0','aria-label':t('upsT')},cardHead(t('upsT'),h('button',{type:'button',class:'b sm',onclick:openUpstream},t('upAdd'))),
(S.upstreams||[]).length?S.upstreams.map(u=>{const x=lat[u.name];return h('div',{class:'irow'},dot(!x?'var(--ink2)':x.ok?'var(--ok)':'var(--acc)'),h('span',{class:'tt grow'},h('b',null,u.name),h('small',{class:'num ell'},[u.company,upAddr(u),t('nClients',u.clients)].filter(Boolean).join(' · '))),
h('span',{class:'num muted',style:'font-weight:700;font-size:12px;white-space:nowrap'},x?(x.ok?ms1(x.rtt_ms,1):t('noLink')):t('measuring')),
h('button',{type:'button',class:'b sm ghost red',onclick:()=>confirmAsk(t('upDelT',u.name),t('upDelX'),t('disconnect'),async()=>{await api('upstreams/delete',{name:u.name});flash(t('upDeleted'));refresh()},true)},t('disconnect')))}):h('div',{class:'irow muted'},t('noUps')));
const rels=h('section',{class:'card p0','aria-label':t('relsT')},cardHead(t('relsT'),h('button',{type:'button',class:'b sm',onclick:openRelay},t('relAdd'))),
(S.relays||[]).length?S.relays.map(r=>h('div',{class:'irow'},dot(r.online?'var(--ok)':'var(--ink3)'),h('span',{class:'tt grow'},h('b',null,r.name),h('small',null,t('relSub',r.online?t('online'):t('offline'),t('nClients',r.clients),fdate(r.created)))),r.admin?h('span',{class:'adm'},t('admBadge')):null,
h('button',{type:'button',class:'b sm ghost red',onclick:()=>confirmAsk(t('relDelT',r.name),t('relDelX'),t('revoke'),async()=>{await api('relays/delete',{name:r.name});flash(t('relDeleted'));refresh()},true)},t('revoke')))):h('div',{class:'irow muted'},t('noKeys')));
return [sch,h('div',{class:'g2'},ups,rels)]}
async function optimize(){U.opt=true;render();try{const r=await api('cascade/optimize',{});const ev=r.events||[];flash(ev.length?ev.map(e=>e.error?(e.client+': '+e.error):(e.client+': '+e.from+' → '+e.to+' ('+(e.old_ms<0?t('unreachable'):ms1(e.old_ms))+' → '+ms1(e.new_ms)+')')).join('\n'):t('optNone'),false,ev.length?6000:4000);await refresh()}catch(e){flash(e.message,true)}finally{U.opt=false;render()}}
function openUpstream(){const M={title:t('upTitle'),text:t('upText'),okLabel:t('connect'),up:{name:'',link:''}};
M.body=()=>[fld(t('nodeName'),inp('up-name',M.up.name,v=>{M.up.name=v},{maxlength:40})),fld(t('upKey'),inp('up-link',M.up.link,v=>{M.up.link=v},{area:1,cls:'mono',placeholder:'veil-relay://…',spellcheck:'false',style:'height:96px'}))];
M.run=async()=>{await api('upstreams',{name:M.up.name,link:M.up.link});flash(t('upDone'));refresh()};ask(M)}
function openRelay(){const M={title:t('relTitle'),text:t('relText'),okLabel:t('issue'),rl:{name:'',admin:false}};
M.body=()=>[fld(t('relFor'),inp('rl-name',M.rl.name,v=>{M.rl.name=v},{maxlength:40})),swRow(M.rl.admin,t('relAdminL'),t('relAdminSub'),()=>{M.rl.admin=!M.rl.admin;render()}),M.rl.admin?h('div',{class:'dng'},t('relAdminWarn')):null];
M.run=async()=>{const r=await api('relays',{name:M.rl.name,admin:M.rl.admin});showLink(t('relKeyTitle',r.name),r,t('relKeyWarn',r.name));refresh()};ask(M)}

/* ---------- Протоколы ---------- */
const PROTOS=[{k:'wireguard',name:'WireGuard',ctr:'veil-wireguard',d:'dWG'},{k:'amneziawg',name:'AmneziaWG',ctr:'veil-amneziawg',d:'dAWG'},{k:'openvpn',name:'OpenVPN',ctr:'veil-openvpn',d:'dOVPN'},{k:'ipsec',name:'IPsec/IKEv2',ctr:'veil-ipsec',d:'dIPSEC'}];
function restartBtn(c,label,txt){return h('button',{type:'button',class:'b',onclick:()=>confirmAsk(t('rsT',c.name),txt||t('rsX',label),t('restart'),async()=>{await api('admin/protocols/restart',{name:c.name});flash(t('rsDone',c.name));opt('admin/protocols').then(v=>{PROTO=v;render()})})},t('restart'))}
function protoClient(c,p){const M={title:t('pcT',p.name),text:t('pcX'),field:'',ph:t('pcPh'),okLabel:t('create')};M.run=async()=>{const n=(M.field||'').trim();const r=await api('admin/protocols/client',{name:c.name,client:n});showLink(t('pcTitle',p.name,n),{text:r.text,file:r.file||(n+'.conf')},t('pcWarn'))};ask(M)}
function pcard(o){return h('section',{class:'pcard'},h('div',{class:'ph'},dot(o.dot),h('b',null,o.name),h('span',{class:'num muted',style:'font-size:11px'},o.ctr)),h('span',{class:'note'},o.desc),
h('div',{class:'row',style:'gap:16px;font-size:12px;font-weight:500;color:var(--ink2)'},h('span',null,t('port')+': ',h('b',{class:'num',style:'color:var(--ink)'},o.port)),o.clients!==undefined?h('span',null,t('clientsL')+': ',h('b',{class:'num',style:'color:var(--ink)'},o.clients)):null,o.state?h('span',{class:'ell'},o.state):null),
h('div',{class:'acts'},o.acts))}
function pgProtocols(){const out=[h('p',{class:'note big'},t('protoIntro'))];
if(PROTO===null)out.push(h('div',{class:'box'},t('loading')));else if(PROTO===false)out.push(h('div',{class:'box'},t('protoNA')));else if(!PROTO.docker)out.push(h('div',{class:'box'},t('protoNoDocker')));
const can=!!(PROTO&&PROTO.docker);const cs=ctrs();const cards=[];
const vc=cs.find(c=>c.name==='veil');cards.push(pcard({name:'Veil',ctr:'veil',dot:vc?ctrDot(vc):'var(--ok)',state:vc?(vc.status||vc.state):t('running'),desc:t('dVeil'),port:S.server.port+'/tcp+udp',clients:String(S.clients.length),acts:can&&vc?[restartBtn(vc,'Veil',t('rsXVeil'))]:[]}));
const ac=cs.find(c=>c.name==='veil-admin');if(ac)cards.push(pcard({name:lang==='ru'?'Админ-панель':'Admin panel',ctr:ac.name,dot:ctrDot(ac),state:ac.status||ac.state,desc:t('dAdmin'),port:t('noPorts'),acts:can?[restartBtn(ac,'',t('rsXAdmin'))]:[]}));
PROTOS.forEach(p=>{const c=cs.find(x=>x.protocol===p.k||x.name===p.ctr);
cards.push(pcard({name:p.name,ctr:c?c.name:p.ctr,dot:ctrDot(c),state:c?(c.status||c.state):t('notInstalled'),desc:t(p.d),port:c?(c.ports||'—'):'—',
acts:c?(can?[c.state==='running'?h('button',{type:'button',class:'b',onclick:()=>protoClient(c,p)},t('addClient')):null,restartBtn(c,p.name)]:[]):[h('span',{class:'note'},t('installFromApp'))]}))});
out.push(h('div',{class:'g3'},cards),h('p',{class:'note'},t('updNote')));return out}

/* ---------- Сайт-прикрытие ---------- */
function coverVal(){const r=S.site_repo||{};const b={mode:r.enabled?'repo':'gen',url:r.url||'',ref:r.ref||'',dir:r.dir||'',token:''};for(const k in U.cover)b[k]=U.cover[k];return b}
async function coverSave(){const r=S.site_repo||{},v=coverVal();const url=v.url.trim();if(!url){flash(t('srNeedUrl'),true);return}
const same=r.enabled&&url===r.url&&v.ref.trim()===(r.ref||'')&&v.dir.trim()===(r.dir||'')&&!v.token;U.applying=true;render();
try{if(same)await api('settings/siterepo/refresh',{});else await api('settings/siterepo',{url,ref:v.ref.trim(),dir:v.dir.trim(),token:v.token});U.cover={mode:'repo'};flash(t('srQueued'));await refresh()}catch(e){flash(e.message,true)}finally{U.applying=false;render()}}
function pgCover(){const s=S.server,r=S.site_repo||{},v=coverVal();
const modes=[['gen','cvGen','cvGenSub'],['repo','cvRepo','cvRepoSub']].map(m=>radio(v.mode===m[0],t(m[1]),t(m[2]),()=>{U.cover.mode=m[0];render()}));
const left=h('section',{class:'card','aria-label':t('cvT')},h('h2',null,t('cvT')),h('div',{class:'col',role:'radiogroup'},modes));
if(v.mode==='repo'){const same=r.enabled&&v.url.trim()===r.url&&v.ref.trim()===(r.ref||'')&&v.dir.trim()===(r.dir||'')&&!v.token;
left.append(h('div',{class:'g3f'},fld(t('srUrl'),inp('cv-url',v.url,x=>{U.cover.url=x},{cls:'mono',placeholder:'https://github.com/…',spellcheck:'false',live:1})),fld(t('srRef'),inp('cv-ref',v.ref,x=>{U.cover.ref=x},{cls:'mono',placeholder:'main',spellcheck:'false',live:1})),fld(t('srDir'),inp('cv-dir',v.dir,x=>{U.cover.dir=x},{cls:'mono',placeholder:'public',spellcheck:'false',live:1}))),
h('div',{class:'row',style:'align-items:flex-end'},h('div',{class:'grow'},fld(t('srToken'),inp('cv-tok',v.token,x=>{U.cover.token=x},{type:'password',autocomplete:'new-password',placeholder:r.has_token?'••••••••••••':'',live:1}))),
r.enabled&&r.has_token?h('button',{type:'button',class:'b red',onclick:()=>confirmAsk(t('srClrT'),t('srClrX'),t('srClrTok'),async()=>{await api('settings/siterepo',{url:r.url,ref:r.ref||'',dir:r.dir||'',clear_token:true});flash(t('srClrDone'));refresh()},true)},t('srClrTok')):null));
if(r.syncing)left.append(h('div',{class:'pbar'},h('i')));
if(r.enabled&&r.error&&!r.syncing)left.append(h('div',{class:'dng'},t('srErr',r.error)));
left.append(h('div',{class:'row',style:'font-size:12px;font-weight:500;color:var(--ink2)'},h('span',{class:'grow'},!r.enabled?'':r.syncing?t('srBusy'):r.active&&r.updated?t('srOk',ago(r.updated))+(r.commit?' · '+t('srMeta',String(r.commit).slice(0,8),r.files||0):''):t('srWait')),
h('button',{type:'button',class:'b '+(same?'':'acc'),disabled:U.applying||r.syncing,onclick:coverSave},same?t('srRefresh'):t('srSave'))))}
else{left.append(h('div',{class:'kv'},h('span',null,t('company')),h('span',{style:'font-weight:600'},s.company||'—'),h('span',null,t('domainL')),h('span',{class:'num'},s.domain||'—')),h('p',{class:'note'},t('cvGenNote')));
if(r.enabled)left.append(h('div',null,h('button',{type:'button',class:'b acc',onclick:()=>confirmAsk(t('cvBackT'),t('cvBackX'),t('cvBack'),async()=>{await api('settings/siterepo',{url:''});U.cover={};flash(t('cvBackDone'));refresh()},true)},t('cvBack'))))}
const repoOn=r.enabled&&r.active;
const pv=h('section',{class:'card','aria-label':t('pvT')},h('h2',null,t('pvT')),repoOn?h('div',{class:'box',style:'min-height:120px'},h('b',{style:'display:block;color:var(--ink);margin-bottom:6px'},t('pvRepo')),h('span',{class:'num'},r.url+(r.dir?' / '+r.dir:''))):
h('div',{class:'pv'},h('span',{class:'pt'},s.company||'—'),h('span',{class:'ph'},t('pvHead')),h('span',{class:'ps'},t('pvSub')),h('span',{class:'pb'},t('pvBtn'))));
return h('div',{class:'gcv'},left,pv)}

/* ---------- Настройки ---------- */
const FRONTS=['ya.ru','vk.com','mail.ru','yandex.ru'];
function setBase(){const s=S.server;return {v4:s.public_ipv4||'',v6:s.public_ipv6||'',domain:s.domain||'',front:s.front&&s.front.enabled?s.front.sni||'':''}}
function setVal(){const b=setBase();for(const k in U.set)if(k!=='custom')b[k]=U.set[k];return b}
function setDirtyN(){const b=setBase(),v=setVal();return ['v4','v6','domain','front'].filter(k=>(v[k]||'').trim()!==(b[k]||'').trim()).length}
async function applySettings(){const b=setBase(),v=setVal();const custom=U.set.custom||(v.front&&FRONTS.indexOf(v.front)<0);if(custom&&!v.front.trim()){flash(t('frBad'),true);return}
const run=async()=>{U.applying=true;render();try{if(v.v4.trim()!==b.v4||v.v6.trim()!==b.v6)await api('settings/addresses',{ipv4:v.v4.trim(),ipv6:v.v6.trim()});
if(v.domain.trim()!==b.domain)await api('settings/domain',{domain:v.domain.trim()});
const fr=v.front.trim().toLowerCase();if(fr!==b.front){const cur=S.server.front||{};await api('settings/front',{sni:fr,dest:fr&&fr===cur.sni?cur.dest||'':''})}
U.set={};flash(t('setApplied'));await refresh()}catch(e){flash(e.message,true);await refresh()}finally{U.applying=false;render()}};
const fr=v.front.trim();if(fr&&fr!==b.front)confirmAsk(t('frAskT',fr),t('frAskX'),t('enable'),run,true);else run()}
function pgSettings(){const s=S.server,b=setBase(),v=setVal();const custom=!!U.set.custom||(v.front!==''&&FRONTS.indexOf(v.front)<0);
const certTxt=v.domain.trim()!==b.domain?(v.domain.trim()?t('certAfter'):t('certNone')):!b.domain?t('certNone'):({none:t('certNone'),pending:t('certPending'),ready:t('certReady',fdate(s.cert&&s.cert.expires)),error:(s.cert&&s.cert.message)||t('certErr')}[s.cert&&s.cert.state]||'');
const fch=[['',t('frNone')]].concat(FRONTS.map(f=>[f,f]));
const fr=h('section',{class:'card','aria-label':t('frT')},h('h2',null,t('frT')),h('div',{class:'row'},fch.map(f=>chip(f[1],!custom&&v.front===f[0],()=>{U.set.front=f[0];U.set.custom=false;render()},'in')),chip(t('frOther'),custom,()=>{U.set.custom=true;if(FRONTS.indexOf(v.front)>=0||!v.front)U.set.front='';render()},'in')),
custom?inp('st-front',v.front,x=>{U.set.front=x},{cls:'mono',placeholder:'example.ru',live:1,'aria-label':t('frOther')}):null,
v.front.trim()&&v.front.trim()!==b.front?h('div',{class:'dng'},t('frWarn')):null,
b.front&&v.front===b.front?h('p',{class:'note'},t('frOn',b.front)):null,b.front&&b.domain&&v.front===b.front?h('p',{class:'note'},t('frDomainNote')):null);
const vc=ctrs().find(c=>c.name==='veil');const can=PROTO&&PROTO.docker&&vc;
const mt=h('section',{class:'card','aria-label':t('mtT')},h('h2',null,t('mtT')),h('div',{class:'kv'},h('span',null,t('version')),h('span',{class:'num'},s.version||'—'),ME&&ME.mode?[h('span',null,t('mode')),h('span',null,ME.mode==='container'?t('modeContainer'):t('modeBuiltin'))]:null,s.admin_url?[h('span',null,t('adminUrl')),h('span',{class:'num'},s.admin_url)]:null),
h('div',{class:'row'},h('button',{type:'button',class:'b',onclick:backup},t('backup')),can?h('button',{type:'button',class:'b red',onclick:()=>confirmAsk(t('rsT','veil'),t('rsXVeil'),t('restart'),async()=>{await api('admin/protocols/restart',{name:vc.name});flash(t('rsDone','veil'))},true)},t('restartVeil')):null),h('p',{class:'note'},t('updNote')));
return h('div',{class:'g2'},
h('section',{class:'card','aria-label':t('adrT')},h('h2',null,t('adrT')),h('p',{class:'note'},t('adrX')),fld('IPv4',inp('st-v4',v.v4,x=>{U.set.v4=x},{cls:'mono',live:1,placeholder:'203.0.113.10'})),fld('IPv6',inp('st-v6',v.v6,x=>{U.set.v6=x},{cls:'mono',live:1,placeholder:'2001:db8::10'}))),
h('section',{class:'card','aria-label':t('domT')},h('h2',null,t('domT')),fld(t('domainL'),inp('st-dom',v.domain,x=>{U.set.domain=x},{cls:'mono',live:1,placeholder:'vpn.example.com',autocomplete:'off'})),h('div',{class:'box'},certTxt)),fr,mt)}
function backup(){confirmAsk(t('bkT'),t('bkX'),t('download'),async()=>{const r=await fetch(apiURL('backup'),{headers:H});if(!r.ok){let j={};try{j=await r.json()}catch(e){}throw new Error(r.status===404?t('bkNA'):(j.error||t('errN',r.status)))}
const cd=r.headers.get('Content-Disposition')||'';const m=cd.match(/filename="?([^";]+)"?/);const name=m?m[1]:'veil-backup-'+today()+'.json';download(name,await r.blob());flash(t('bkDone',name),false,5000)})}

/* ---------- Безопасность ---------- */
function pwState(){const p=U.pw,needCur=!(ME&&ME.role_login);let msg=t('pwHint'),ink='var(--ink2)',bad=true;
if(p.n1&&p.n1.length<8){msg=t('pwShort');ink='var(--accInk)'}else if(p.n2&&p.n1!==p.n2){msg=t('pwMismatch');ink='var(--accInk)'}else if((p.cur||!needCur)&&p.n1&&p.n1===p.n2){bad=false;msg=t('pwReady');ink='var(--ok)'}return {msg,ink,bad,needCur}}
async function savePw(){const st=pwState();if(st.bad||U.applying)return;U.applying=true;render();try{await api('settings/password',{current:U.pw.cur,new:U.pw.n1});U.pw={cur:'',n1:'',n2:''};flash(t('pwDone'),false,1500);setTimeout(()=>{location.href='/admin'},1500)}catch(e){flash(e.message,true)}finally{U.applying=false;render()}}
function pgSecurity(){const st=pwState();const adm=S.clients.filter(c=>c.role==='admin').map(c=>c.name);const me=ME&&ME.user||'admin';
const pw=h('section',{class:'card','aria-label':t('pwT')},h('h2',null,t('pwT')),st.needCur?fld(t('pwCur'),inp('pw-cur',U.pw.cur,x=>{U.pw.cur=x},{type:'password',autocomplete:'current-password',live:1})):h('p',{class:'note'},t('pwRoleNote')),
fld(t('pwNew'),inp('pw-n1',U.pw.n1,x=>{U.pw.n1=x},{type:'password',autocomplete:'new-password',live:1})),fld(t('pwRep'),inp('pw-n2',U.pw.n2,x=>{U.pw.n2=x},{type:'password',autocomplete:'new-password',live:1,onkeydown:e=>{if(e.key==='Enter')savePw()}})),
h('span',{style:'font-size:12px;font-weight:600;color:'+st.ink,role:'status'},st.msg),h('div',null,h('button',{type:'button',class:'b lg '+(st.bad?'':'acc'),disabled:st.bad||U.applying,onclick:savePw},t('pwSave'))));
const pol=h('section',{class:'card','aria-label':t('polT')},h('h2',null,t('polT')),h('div',{class:'kv'},h('span',null,t('polFrom')),h('span',null,t('polFromV')),h('span',null,t('polAdm')),h('span',null,t('polAdmV',adm.length?adm.join(', '):t('none'))),h('span',null,t('polUsr')),h('span',null,t('polUsrV')),h('span',null,t('polFail')),h('span',null,t('polFailV')),h('span',null,t('polSess')),h('span',null,t('polSessV'))));
const ss=ASESS&&ASESS.sessions||[];
const as=h('section',{class:'card p0','aria-label':t('asT')},cardHead(t('asT'),ASESS&&ss.length>1?h('button',{type:'button',class:'b sm red',onclick:async()=>{try{await api('admin/sessions/kill',{all_others:true});flash(t('asKilled'));ASESS=await opt('admin/sessions');render()}catch(e){flash(e.message,true)}}},t('asKillAll')):null),
ASESS===null?h('div',{class:'irow muted'},t('loading')):!ASESS?h('div',{class:'irow muted'},t('asNA')):ss.map(x=>h('div',{class:'irow h48'},h('span',{class:'ell',style:'font-weight:600;width:200px;flex:none'},me+(x.current?' ('+t('asYou')+')':'')),h('span',{class:'num',style:'font-size:12px;color:var(--hi);width:140px;flex:none'},x.ip||'—'),h('span',{class:'muted grow ell',title:x.ua||''},x.ua||'—'),
h('span',{style:'font-size:12px;font-weight:600;white-space:nowrap;color:'+(x.current?'var(--ok)':'var(--ink2)')},x.current?t('now'):ago(x.last)),x.current?null:h('button',{type:'button',class:'b sm ghost red',onclick:async()=>{try{await api('admin/sessions/kill',{id:x.id});flash(t('asKilled1'));ASESS=await opt('admin/sessions');render()}catch(e){flash(e.message,true)}}},t('asKill')))));
const bl=BLK&&BLK.blocked||[];
const blk=h('section',{class:'card p0','aria-label':t('blT')},cardHead(t('blT')),BLK===null?h('div',{class:'irow muted'},t('loading')):!BLK?h('div',{class:'irow muted'},t('blNA')):bl.length?bl.map(b=>{const c=S.clients.find(x=>x.address4===b.ip||x.address6===b.ip);
return h('div',{class:'irow h48'},h('span',{class:'num',style:'width:200px;flex:none'},b.ip),h('span',{class:'muted grow'},(c?c.name+' · ':'')+t('blSub',b.fails,fdt(b.until,true))),h('button',{type:'button',class:'b sm',onclick:async()=>{try{await api('admin/unblock',{ip:b.ip});flash(t('blDone'));BLK=await opt('admin/blocked');render()}catch(e){flash(e.message,true)}}},t('unblock')))}):h('div',{class:'irow muted'},t('blNone')));
return [h('div',{class:'g2'},pw,pol),as,blk]}
const PAGES={overview:pgOverview,sessions:pgSessions,audit:pgAudit,clients:pgClients,links:pgLinks,cascade:pgCascade,protocols:pgProtocols,cover:pgCover,settings:pgSettings,security:pgSecurity};

/* ---------- конструктор каскада ---------- */
const NS='http://www.w3.org/2000/svg';
function qual(r){return r<0?'bad':(r<100?'good':(r<200?'mid':'bad'))}
const QCOL={good:'var(--ok)',mid:'var(--ink2)',bad:'var(--accInk)'};
function buildGraph(route,editable,sel,canvas,size,avail,vertical){
const lat=latMap(),s=S.server;
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
if(idx<last){let q,label;
if(idx===0){q=s.online>0?'good':'mid';label='—'}
else if(seq[idx+1].k==='net'){q='good';label=t('exit')}
else{const r=idx===1?rttOf(seq[idx+1]):null;q=r===null?'mid':qual(r);label=r===null?'—':ms1(r)}
const q2=pos[idx+1],a=vertical?{x:p.x+NW/2,y:p.y+NH}:{x:p.x+NW,y:p.y+20},b=vertical?{x:q2.x+NW/2,y:q2.y}:{x:q2.x,y:q2.y+20};
const d=vertical?'M'+a.x+' '+a.y+' C'+a.x+' '+(a.y+gy*0.6)+','+b.x+' '+(b.y-gy*0.6)+','+b.x+' '+b.y:'M'+a.x+' '+a.y+' C'+(a.x+gx*0.6)+' '+a.y+','+(b.x-gx*0.6)+' '+b.y+','+b.x+' '+b.y;
const path=document.createElementNS(NS,'path');path.setAttribute('d',d);path.setAttribute('fill','none');path.setAttribute('stroke',QCOL[q]);path.setAttribute('stroke-width','2.5');path.setAttribute('stroke-dasharray',q!=='bad'?'3 9':'5 6');path.setAttribute('stroke-linecap','round');if(q!=='bad')path.style.animation='aFlow 1s linear infinite';svg.append(path);
const lb=el('span','glabel');lb.style.cssText='left:'+((a.x+b.x)/2-38)+'px;top:'+((a.y+b.y)/2-13)+'px';lb.append(el('b','',label));canvas.append(lb)}});
const sc=(editable||vertical)?1:Math.max(0.72,Math.min(1,avail/W));
canvas.style.transform='scale('+sc+')';size.style.width=Math.round(W*sc)+'px';size.style.height=Math.round(Hh*sc)+'px'}
let bchain=[],bmode='manual',bsel='this';
function bOrder(){let r=bchain.slice();if(bmode==='auto'&&r.length>2){const lat=latMap();const k=n=>{const x=lat[n];return x&&x.ok?x.rtt_ms:1e9};r=[r[0]].concat(r.slice(1,-1).sort((a,b)=>k(a)-k(b)),[r[r.length-1]])}return r}
function openBuilder(){const c=pickClient();bchain=c?c.route.filter(n=>upInfo(n)):[];bmode=c&&c.auto?'auto':'manual';bsel='this';applyStatic();$('builder').classList.add('show');renderBuilder()}
function renderBuilder(){if(!S)return;const lat=latMap(),s=S.server;const ord=bOrder();const key=bsel==='this'||ord.includes(bsel)?bsel:'this';bsel=key;
const m=$('b-modes');m.replaceChildren();[['manual',t('mManual')],['auto',t('mAuto')]].forEach(x=>{m.append(btn(x[1],bmode===x[0]?'on':'',()=>{bmode=x[0];renderBuilder()}))});
const pal=$('b-pal');pal.replaceChildren();
(S.upstreams||[]).filter(u=>!bchain.includes(u.name)).forEach(u=>{const x=lat[u.name];const bad=!x||!x.ok;const d=el('div','pitem'+(bad?' off':''));const l=el('div');
l.append(el('strong','',u.name),el('div','',upAddr(u)+' · '+(x?(x.ok?ms1(x.rtt_ms):t('noLink')):t('measuring'))));
const b=btn('+','',()=>{if(bad)return;if(bchain.length>=1)bchain.splice(bchain.length-1,0,u.name);else bchain.push(u.name);renderBuilder()});b.setAttribute('aria-label',t('addRoute',u.name));d.append(l,b);pal.append(d)});
buildGraph(ord,true,key,$('b-canvas'),$('b-size'),0,window.innerWidth<640);
const wr=$('b-warns');wr.replaceChildren();const w=(cls,x)=>wr.append(el('div',cls,x));
const offs=ord.filter(n=>{const x=lat[n];return !x||!x.ok});
if(ord.length+1>5)w('bad',t('wMany'));if(offs.length)w('bad',t('wOff',offs.join(', ')));if(bmode==='auto')w('info',t('wAuto'));if(!wr.children.length)w('ok',t('wOk'));
const idx=key==='this'?0:ord.indexOf(key)+1,n=ord.length+1,isUp=key!=='this';const u=isUp?upInfo(key):null,x=isUp?lat[key]:null;
$('i-title').textContent=isUp?key:t('thisServer');
const rr=[[t('role'),n===1?t('entryExit'):(idx===0?t('entry'):(idx===n-1?t('exit'):t('inter')))],[t('address'),isUp?upAddr(u):srvAddr()]];
if(isUp){if(u&&u.company)rr.push([t('company'),u.company]);rr.push([t('latFromThis'),x?(x.ok?ms1(x.rtt_ms):'—'):'—'])}else rr.push([t('transport'),'HTTP/3 → HTTP/2']);
rr.push([t('stClients'),isUp?(u?String(u.clients):'—'):String(s.clients)],[t('position'),(idx+1)+' / '+n]);
const ir=$('i-rows');ir.replaceChildren();rr.forEach(r=>{const d=el('div','irw');d.append(el('span','',r[0]),el('b','',r[1]));ir.append(d)});
$('i-bars').parentNode.style.display=isUp?'block':'none';const bars=$('i-bars');bars.replaceChildren();const hs=isUp?(hist[key]||[]):[];$('i-histcap').textContent=isUp?t('histCap'):'';
if(isUp&&!hs.length)bars.append(el('span','muted',t('noHistory')));
const mx=Math.max.apply(null,hs.concat([1]));hs.forEach((v,i)=>{const b=el('div',v<0?'fail':(i===hs.length-1?'last':''));b.style.height=(v<0?8:Math.max(8,Math.round(100*v/mx)))+'%';bars.append(b)});
$('i-route').textContent=[t('clients'),t('thisServer')].concat(ord,[t('internet')]).join(' → ');
const f=ord.length?lat[ord[0]]:null;$('i-total').textContent=(f&&f.ok?t('routeTotalFirst',ms1(f.rtt_ms))+' · ':t('routeTotalUnk')+' · ')+n+' '+t('nodesShort')}
$('b-byKey').onclick=openUpstream;$('b-issueKey').onclick=openRelay;
$('b-close').onclick=()=>$('builder').classList.remove('show');
$('b-check').onclick=async()=>{const ord=bOrder();try{await api('explore',{route:ord});flash(t('checkOk',ord.length+1))}catch(e){flash(e.message,true)}};
$('b-apply').onclick=()=>{const ord=bOrder();if(!ord.length){flash(t('applyEmpty'),true);return}$('builder').classList.remove('show');openNewClient(bmode==='auto'&&ord.length>1?{mode:'auto',exit:ord[ord.length-1],hops:ord.length-1}:{mode:'manual',route:ord})};

/* ---------- запуск ---------- */
document.addEventListener('click',()=>{if(U.dd||U.nodeOpen){U.dd=null;U.nodeOpen=false;render()}});
document.addEventListener('keydown',e=>{if(e.key!=='Escape')return;if(U.dd||U.nodeOpen){U.dd=null;U.nodeOpen=false;render();return}
if(U.modal){if(U.modal.kind!=='link'&&!U.modal.busy)closeModal();return}if(U.drawer){closeDrawer();return}if($('builder').classList.contains('show'))$('builder').classList.remove('show')});
window.addEventListener('hashchange',()=>{const p=location.hash.slice(1);if(PG[p]&&p!==U.page)go(p)});
let rz=0;window.addEventListener('resize',()=>{clearTimeout(rz);rz=setTimeout(()=>{if(S&&$('builder').classList.contains('show'))renderBuilder()},120)});
(function(){const p=location.hash.slice(1);if(PG[p])U.page=p})();
applyStatic();render();refresh();
setInterval(()=>{if(U.auto&&!document.hidden)refresh()},5000);
opt('cascade/nodes').then(r=>{if(r){NODES=r.nodes||[];render()}});
</script>
</body></html>`
