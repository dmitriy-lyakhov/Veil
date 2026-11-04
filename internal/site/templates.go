package site

const templates = `
{{define "head"}}<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Company}}{{if .Tagline}} — {{.Tagline}}{{end}}</title>
<meta name="description" content="{{.Tagline}}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="stylesheet" href="/assets/site.css">
</head>{{end}}

{{define "brand"}}<a class="brand" href="/"><span class="logo">{{.Initials}}</span><span>{{.Company}}</span></a>{{end}}

{{define "index"}}{{template "head" .}}
<body class="l{{.Layout}}">
<header class="top"><div class="wrap">{{template "brand" .}}<nav><a href="#services">{{.T "services"}}</a><a href="#about">{{.T "about"}}</a><a href="#contact" class="btn small">{{.T "cta"}}</a></nav></div></header>
<section class="hero"><div class="wrap">
<div class="hero-text"><h1>{{.Tagline}}</h1><p>{{index .About 0}}</p><a class="btn" href="#contact">{{.T "cta"}}</a></div>
{{if ne .Layout 0}}<div class="hero-card"><span class="logo big">{{.Initials}}</span><strong>{{.Company}}</strong><span>{{.City}} · {{.Since}}</span></div>{{end}}
</div></section>
<main class="wrap">
{{range .Order}}
{{if eq . "services"}}<section id="services"><h2>{{$.T "services"}}</h2><div class="grid">{{range $.Services}}<div class="card"><h3>{{.Title}}</h3><p>{{.Text}}</p></div>{{end}}</div></section>{{end}}
{{if eq . "about"}}<section id="about"><h2>{{$.T "about"}}</h2>{{range $.About}}<p class="lead">{{.}}</p>{{end}}</section>{{end}}
{{if eq . "why"}}<section id="why"><h2>{{$.T "why"}}</h2><div class="grid three">{{range $.Values}}<div class="value"><h3>{{.Title}}</h3><p>{{.Text}}</p></div>{{end}}</div></section>{{end}}
{{end}}
<section id="contact" class="contact"><div><h2>{{.T "contact"}}</h2><dl><dt>{{.T "city"}}</dt><dd>{{.City}}</dd><dt>{{.T "hours"}}</dt><dd>{{.Hours}}</dd></dl></div>
<form method="post" action="/contact"><input name="name" placeholder="{{.T "name"}}" required maxlength="80"><input name="reply" placeholder="{{.T "phone_or_email"}}" required maxlength="120"><textarea name="message" rows="4" placeholder="{{.T "message"}}" maxlength="2000"></textarea><button class="btn" type="submit">{{.T "send"}}</button></form></section>
</main>
<footer><div class="wrap">© {{.Since}}–{{.Year}} {{.Company}}. {{.T "rights"}}.</div></footer>
</body></html>{{end}}

{{define "thanks"}}{{template "head" .}}
<body class="l{{.Layout}} plain"><header class="top"><div class="wrap">{{template "brand" .}}</div></header>
<main class="wrap center"><h1>{{.T "thanks_title"}}</h1><p class="lead">{{.T "thanks"}}</p><a class="btn" href="/">{{.T "back"}}</a></main></body></html>{{end}}

{{define "login"}}{{template "head" .}}
<body class="l{{.Layout}} plain"><main class="login"><form method="post" action="/admin/login" class="card">
<div class="brand">{{template "brand" .}}</div><h2>{{.T "signin"}}</h2><p class="muted">{{.T "signin_hint"}}</p>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<input name="login" placeholder="{{.T "login"}}" autocomplete="username" required>
<input name="password" type="password" placeholder="{{.T "password"}}" autocomplete="current-password" required>
<button class="btn" type="submit">{{.T "enter"}}</button></form></main></body></html>{{end}}

{{define "favicon"}}<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="14" fill="{{.P.Primary}}"/><text x="32" y="42" font-family="Arial,Helvetica,sans-serif" font-size="28" font-weight="700" fill="#fff" text-anchor="middle">{{.Initials}}</text></svg>{{end}}

{{define "css"}}:root{--p:{{.P.Primary}};--a:{{.P.Accent}};--ink:{{.P.Ink}};--soft:{{.P.Soft}};--hero:{{.P.Hero}}}
*{box-sizing:border-box}body{margin:0;font:16px/1.6 system-ui,-apple-system,"Segoe UI",Roboto,Arial,sans-serif;color:var(--ink);background:#fff}
a{color:var(--p);text-decoration:none}.wrap{max-width:1080px;margin:0 auto;padding:0 20px}
.top{position:sticky;top:0;background:rgba(255,255,255,.95);border-bottom:1px solid #e6e9ee;z-index:2}.top .wrap{display:flex;align-items:center;justify-content:space-between;height:64px}
.brand{display:flex;align-items:center;gap:10px;font-weight:700;color:var(--ink)}nav{display:flex;gap:22px;align-items:center}nav a{color:var(--ink)}
.logo{display:inline-grid;place-items:center;width:34px;height:34px;border-radius:9px;background:var(--p);color:#fff;font-weight:800;font-size:14px}.logo.big{width:72px;height:72px;font-size:28px;border-radius:18px}
.btn{display:inline-block;background:var(--a);color:#fff;border:0;padding:12px 22px;border-radius:10px;font-weight:600;cursor:pointer;font-size:15px;font-family:inherit;text-decoration:none}.btn.small{padding:8px 14px}
.hero{background:var(--hero);color:#fff;padding:84px 0}.hero .wrap{display:flex;gap:40px;align-items:center}.hero-text{min-width:0}.hero h1{font-size:40px;line-height:1.15;margin:0 0 16px}.hero p{opacity:.85;max-width:620px;margin:0 0 20px}
.hero-card{flex:0 0 260px;background:rgba(255,255,255,.08);border:1px solid rgba(255,255,255,.15);border-radius:18px;padding:28px;display:flex;flex-direction:column;gap:10px;align-items:flex-start}
.l1 .hero{background:linear-gradient(135deg,var(--hero),var(--a))}.l2 .hero{background:var(--soft);color:var(--ink)}.l2 .hero p{opacity:1}.l2 .hero-card{background:#fff;border-color:#e6e9ee}
.l2 .hero .wrap{flex-direction:row-reverse}
main section{padding:56px 0;border-bottom:1px solid #eef0f3;animation:sFade .4s both}h2{font-size:28px;margin:0 0 24px}.lead{font-size:18px;max-width:780px;margin:0 0 12px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:18px}.card{background:var(--soft);border-radius:14px;padding:22px}.card h3,.value h3{margin:0 0 8px;font-size:18px}.card p,.value p{margin:0}
.value{border-left:3px solid var(--a);padding:4px 0 4px 16px}
.contact{display:grid;grid-template-columns:1fr 1.3fr;gap:40px;padding:56px 0}dl{display:grid;grid-template-columns:auto 1fr;gap:6px 16px;margin:0}dt{color:#6b7385}dd{margin:0}
form{display:flex;flex-direction:column;gap:12px}input,textarea{font:inherit;padding:12px 14px;border:1px solid #d6dbe3;border-radius:10px;background:#fff;color:#1b2631;width:100%}.contact .btn{align-self:flex-start}
footer{background:var(--ink);color:#c9ced8;padding:28px 0;font-size:14px}
.center{text-align:center;padding:120px 20px;animation:sFade .4s both}.center h1{font-size:36px;margin:0 0 16px}.center .lead{max-width:560px;margin:0 auto 24px}.login{min-height:100vh;display:grid;place-items:center;background:var(--soft);padding:20px}.login .card{width:min(380px,100%);background:#fff;box-shadow:0 10px 40px rgba(0,0,0,.08)}
.login h2{margin:18px 0 0;font-size:24px}.login .card{animation:sFade .4s both}.muted{color:#6b7385;margin:0 0 6px}.error{color:#c0392b;margin:0}
@keyframes sFade{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}@media(prefers-reduced-motion:reduce){*{animation-duration:.001s!important}}
@media(max-width:760px){.hero .wrap,.l2 .hero .wrap{flex-direction:column;align-items:flex-start}.hero h1{font-size:30px}nav a:not(.btn){display:none}.contact{grid-template-columns:1fr}.hero-card{display:none}}
{{end}}
`
