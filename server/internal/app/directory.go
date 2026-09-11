package app

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

// 对外 SEO 插件目录：服务端直出 HTML，可被搜索引擎收录；
// 每个插件页展示一条命令安装方式（Codex 官方插件市场源 + loadout CLI）。
const directoryTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<style>
:root { color-scheme: light; }
body { font-family: system-ui, -apple-system, "PingFang SC", sans-serif; margin: 0; background: #f5f5f7; color: #202124; }
header { background: #fff; border-bottom: 1px solid #d8d9df; padding: 2rem 1.25rem; }
h1 { margin: 0 0 .25rem; font-size: 1.6rem; }
.tagline { color: #62646c; }
main { max-width: 46rem; margin: 0 auto; padding: 1.25rem; }
article { background: #fff; border: 1px solid #d8d9df; border-radius: 12px; padding: 1rem 1.25rem; margin-bottom: 1rem; }
article h2 { margin: 0 0 .25rem; font-size: 1.15rem; }
article h2 a { color: inherit; text-decoration: none; }
article h2 a:hover { text-decoration: underline; }
.kind { display: inline-block; font-size: .75rem; color: #765000; background: #ffe5a0; border-radius: 999px; padding: .1rem .6rem; margin-left: .5rem; vertical-align: 2px; }
.desc { color: #3c4043; margin: .35rem 0 .6rem; }
code, pre { font-family: ui-monospace, Menlo, Consolas, monospace; }
pre { background: #161719; color: #f4f4f7; border-radius: 8px; padding: .75rem 1rem; overflow-x: auto; font-size: .85rem; }
.gw { color: #34674b; font-size: .85rem; }
footer { text-align: center; color: #62646c; padding: 2rem 0 3rem; font-size: .85rem; }
</style>
</head>
<body>
<header><h1>Loadout 插件市场</h1><p class="tagline">一行命令，把专家的整套 AI 装备装进 Codex / Claude Code。</p></header>
<main>
<section style="background:#fff;border:1px solid #d8d9df;border-radius:12px;padding:1rem 1.25rem;margin-bottom:1.25rem">
<h2 style="font-size:1rem;margin:0 0 .5rem">接入整个市场（Codex）</h2>
<pre>codex plugin marketplace add {{.Origin}}/marketplace.git</pre>
</section>
{{range .Plugins}}
<article>
<h2><a href="/plugins/{{.Slug}}">{{.Name}}</a><span class="kind">{{.KindLabel}}</span></h2>
<p class="desc">{{.Description}}</p>
{{if .Gateway}}<p class="gw">服务端独享 · 零配置零密钥 · 按次计量</p>{{end}}
</article>
{{else}}
<article><p class="desc">市场暂时为空。</p></article>
{{end}}
</main>
<footer>Loadout · Your AI, fully loaded.</footer>
</body>
</html>`

const pluginTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} · Loadout 插件市场</title>
<meta name="description" content="{{.Description}}">
<meta property="og:title" content="{{.Name}} · Loadout 插件市场">
<meta property="og:description" content="{{.Description}}">
<style>
body { font-family: system-ui, -apple-system, "PingFang SC", sans-serif; margin: 0; background: #f5f5f7; color: #202124; }
main { max-width: 46rem; margin: 0 auto; padding: 1.5rem 1.25rem 3rem; }
h1 { font-size: 1.5rem; margin: 0 0 .5rem; }
.kind { display: inline-block; font-size: .75rem; color: #765000; background: #ffe5a0; border-radius: 999px; padding: .1rem .6rem; margin-left: .5rem; vertical-align: 4px; }
.desc { color: #3c4043; }
section { background: #fff; border: 1px solid #d8d9df; border-radius: 12px; padding: 1rem 1.25rem; margin-top: 1rem; }
h2 { font-size: 1rem; margin: 0 0 .5rem; }
pre { background: #161719; color: #f4f4f7; border-radius: 8px; padding: .75rem 1rem; overflow-x: auto; font-size: .85rem; }
.gw { color: #34674b; }
a { color: #765000; }
</style>
</head>
<body>
<main>
<h1>{{.Name}}<span class="kind">{{.KindLabel}}</span></h1>
<p class="desc">版本 {{.Version}}</p>
<p class="desc">{{.Description}}</p>
{{if .Gateway}}<p class="gw">服务端独享 · 零配置零密钥 · 按次计量扣费。</p>{{end}}
<section>
<h2>Codex 一行安装</h2>
<pre>codex plugin marketplace add {{.Origin}}/marketplace.git
codex plugin add {{.Slug}} --marketplace loadout</pre>
</section>
<section>
<h2>loadout CLI 安装</h2>
<pre>loadout install {{.Slug}}</pre>
</section>
<p><a href="/plugins">← 返回插件目录</a></p>
</main>
</body>
</html>`

var (
	directoryHTML = template.Must(template.New("directory").Parse(directoryTemplate))
	pluginHTML    = template.Must(template.New("plugin").Parse(pluginTemplate))
)

type directoryPlugin struct {
	Slug, Name, Description, KindLabel, Version string
	Gateway                                     bool
}

func (a *application) pluginDirectory(w http.ResponseWriter, r *http.Request) {
	rows, e := a.s.Pool.Query(r.Context(), "SELECT slug,name,description,kind,transport,version FROM marketplace_items WHERE transport!='stdio' ORDER BY (source='curated') DESC, stars DESC, id")
	if e != nil {
		http.Error(w, "directory unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	plugins := []directoryPlugin{}
	for rows.Next() {
		var item directoryPlugin
		var kind, transport string
		if e = rows.Scan(&item.Slug, &item.Name, &item.Description, &kind, &transport, &item.Version); e != nil {
			http.Error(w, "directory unavailable", http.StatusServiceUnavailable)
			return
		}
		switch kind {
		case "skill":
			item.KindLabel = "技能"
		case "bundle":
			item.KindLabel = "装备组"
		default:
			item.KindLabel = "MCP"
		}
		item.Gateway = transport == "gateway"
		plugins = append(plugins, item)
	}
	if rows.Err() != nil {
		http.Error(w, "directory unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = directoryHTML.Execute(w, map[string]any{
		"Title":       "Loadout 插件市场 · 一行命令武装你的 AI",
		"Description": "Loadout 插件市场：服务端独享 MCP、Agent 技能与专家装备组，一行命令装进 Codex / Claude Code。",
		"Plugins":     plugins,
		"Origin":      a.options.Origin,
	})
}

func (a *application) pluginDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var item directoryPlugin
	var kind, transport string
	e := a.s.Pool.QueryRow(r.Context(), "SELECT slug,name,description,kind,transport,version FROM marketplace_items WHERE slug=$1 AND transport!='stdio'", slug).
		Scan(&item.Slug, &item.Name, &item.Description, &kind, &transport, &item.Version)
	if e == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if e != nil {
		http.Error(w, "directory unavailable", http.StatusServiceUnavailable)
		return
	}
	switch kind {
	case "skill":
		item.KindLabel = "技能"
	case "bundle":
		item.KindLabel = "装备组"
	default:
		item.KindLabel = "MCP"
	}
	item.Gateway = transport == "gateway"
	origin := a.options.Origin
	if origin == "" {
		origin = strings.TrimSuffix(a.options.Origin, "/")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pluginHTML.Execute(w, map[string]any{
		"Name":        item.Name,
		"Slug":        item.Slug,
		"Description": item.Description,
		"KindLabel":   item.KindLabel,
		"Version":     item.Version,
		"Gateway":     item.Gateway,
		"Origin":      origin,
	})
}
