from pathlib import Path
root=Path('/home/yangyang/loadout/web/src')
def edit(path, fn):
 p=root/path; s=p.read_text(); p.write_text(fn(s))
def imp(s): return "import { SidePanel } from '@/components/SidePanel';\n"+s
# Editors own their pending guards and close lifecycle.
for path,title,lock in [('features/operations/ToolsPage.tsx',"item ? t('编辑工具') : t('添加工具')",'save.isPending'),('features/operations/PlansPage.tsx',"item ? t('编辑套餐') : t('添加套餐')",'save.isPending')]:
 def f(s):
  s=imp(s).replace('<section className="editor-panel">',f'<SidePanel title={{{title}}} onClose={{close}} locked={{{lock}}}><section className="editor-panel">',1)
  return s.replace('</section>','</section></SidePanel>',1)
 edit(path,f)
def tools(s):
 s=s.replace("const [expanded, setExpanded] = useState<number | null>(null);", "const [expanded, setExpanded] = useState<Tool | null>(null);")
 start=s.index('              <p>\n                <code>{item.key}</code>')
 end=s.index('              <Button', start)
 s=s[:start]+'''              <div className="tool-summary">
                <span className="tool-cost">{t('{credits} 额度 / 次', { credits: number(item.units_per_call, locale) })}</span>
                <span className="status-badge" data-tone={item.enabled ? 'success' : 'neutral'}>{item.enabled ? t('可用') : t('已停用')}</span>
              </div>
'''+s[end:]
 s=s.replace('aria-expanded={expanded === item.id}', 'aria-haspopup="dialog"').replace('setExpanded(expanded === item.id ? null : item.id)', 'setExpanded(item)')
 start=s.index('              {expanded === item.id && ('); end=s.index('            </div>',start)
 s=s[:start]+s[end:]
 s=s.replace("{admin && editing === undefined && (", "{admin && (")
 s=s.replace('<Pagination label={t(\'工具\')} {...tools.pagination} />', '''<Pagination label={t('工具')} {...tools.pagination} />
      {expanded && <SidePanel title={expanded.name} onClose={() => setExpanded(null)}>
        <p>{expanded.description}</p>
        <div className="tool-summary"><code>{expanded.key}</code><span>{t('{credits} 额度 / 次', { credits: number(expanded.units_per_call, locale) })}</span><span className="status-badge" data-tone={expanded.enabled ? 'success' : 'neutral'}>{expanded.enabled ? t('可用') : t('已停用')}</span></div>
        {admin && expanded.configured && <p>{t('已配置连接')}</p>}
        <section><h3>{t('查看参数')}</h3><pre className="schema-code"><code>{JSON.stringify(expanded.input_schema, null, 2)}</code></pre></section>
        {admin && expanded.kind !== 'builtin' && <ToolMetadataPanel tool={expanded} />}
      </SidePanel>}''')
 return s
edit('features/operations/ToolsPage.tsx',tools)
# Keep add-plan trigger mounted so focus can return.
edit('features/operations/PlansPage.tsx',lambda s:s.replace("{editing === undefined ? (", "{true && (").replace("      ) : (\n        <PlanEditor", "      )}\n      {editing !== undefined && (\n        <PlanEditor"))
edit('features/operations/MarketplacePanel.tsx', lambda s: imp(s).replace('    <section\n      className="editor-panel"', '''    <SidePanel title={t('安装 {value1}', { value1: item.name })} onClose={close} locked={install.isPending}><section
      className="editor-panel"''',1).replace('    </section>','    </section></SidePanel>',1))
edit('features/account/AdminPage.tsx',lambda s: imp(s).replace('    <section\n      className="adjustment-panel"', '''    <SidePanel title={t('调整 {value1} 的余额', { value1: user.username })} onClose={close} locked={adjust.isPending}><section
      className="adjustment-panel"''',1).replace('    </section>', '    </section></SidePanel>',1))
# Self-contained actions: keep mutation state in the page so a failed money operation retains its idempotency key.
edit('features/operations/TeamSettings.tsx',lambda s: imp(s).replace('<section className="editor-panel">',"<SidePanel title={t('团队设置')} trigger={t('团队设置')} locked={save.isPending}><section className=\"editor-panel\">",1).replace('</section>','</section></SidePanel>',1))
def teams(s):
 s=imp(s)
 s=s.replace('<div className="two-column">','<div className="action-row">')
 s=s.replace('        <form\n          className="editor-panel"',"        <SidePanel title={t('新建团队')} trigger={t('新建团队')} locked={create.isPending}><form\n          className=\"editor-panel\"",1)
 s=s.replace('        </form>','        </form></SidePanel>',1)
 pos=s.index('        <form\n          className="editor-panel"')
 s=s[:pos]+s[pos:].replace('        <form\n          className="editor-panel"',"        <SidePanel title={t('接受邀请')} trigger={t('接受邀请')} locked={join.isPending}><form\n          className=\"editor-panel\"",1).replace('        </form>','        </form></SidePanel>',1)
 s=s.replace('<section className="editor-panel">',"<SidePanel title={t('转入团队额度')} trigger={t('转入团队额度')} locked={fund.isPending}><section className=\"editor-panel\">",1).replace('</section>','</section></SidePanel>',1)
 s=s.replace('          <section className="editor-panel">',"          <SidePanel title={t('邀请成员')} trigger={t('邀请成员')} locked={invitation.isPending || Boolean(invite)}><section className=\"editor-panel\">",1)
 marker='          </section>\n        </div>'
 s=s.replace(marker,'          </section></SidePanel>\n        </div>',1)
 return s
edit('features/operations/TeamsPage.tsx',teams)
edit('features/operations/CodesPage.tsx',lambda s: imp(s).replace('      <form',"      <SidePanel title={t('生成兑换码')} trigger={t('生成兑换码')} locked={create.isPending || Boolean(code)}><form",1).replace('      <ErrorNotice error={list.error}', '      </SidePanel>\n      <ErrorNotice error={list.error}',1))
edit('features/account/TokensPage.tsx',lambda s: imp(s).replace('      <form',"      <SidePanel title={t('创建令牌')} trigger={t('创建令牌')} locked={create.isPending || Boolean(secret)}><form",1).replace('      <ErrorNotice error={tokens.error}', '      </SidePanel>\n      <ErrorNotice error={tokens.error}',1))
edit('features/operations/SettingsPage.tsx',lambda s: imp(s).replace('          <form',"          <SidePanel title={t('邮箱验证')} trigger={t('邮箱验证')} locked={send.isPending}><form",1).replace('          </form>', '          </form></SidePanel>',1))
