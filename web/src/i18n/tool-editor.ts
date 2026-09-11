export const toolEditor: Record<string, string> = {
  '只覆盖展示给客户端的工具说明和参数定义，不改变上游执行逻辑或计费。参数字段必须与上游实际接受的输入一致。':
    'Overrides only the descriptions and parameter definitions shown to clients, without changing upstream execution or billing. Parameter fields must match the inputs accepted by the upstream tool.',
  '下面列出服务器发现的实际工具。按工具分别优化说明和参数；清除覆盖后恢复上游定义。连接失败时先检查服务地址、凭证和允许名单，再重试。':
    'These are the actual tools discovered by the server. Customize each description and parameter definition; clearing an override restores the upstream definition. If discovery fails, check the address, credentials and allowlist, then retry.',
  连接类型: 'Connection type',
  结算方式: 'Settlement method',
  协议结算: 'Protocol result',
  业务码匹配: 'Business code match',
  文本匹配: 'Text match',
  脚本判定: 'Script evaluation',
  自定义规则: 'Custom rule',
  免费调用: 'Free calls',
  '先完善工具信息，再连接上游并设置扣费条件。所有配置由服务端保存。':
    'Enter the tool details, connect the upstream service, and set the charging conditions. All settings are saved on the server.',
  基本信息与图标: 'Details and icon',
  '例如：文档搜索': 'For example: Document search',
  '标识用于网关调用，需唯一；使用 3–32 位字母、数字、下划线或连字符。内置工具的标识不可更改。':
    'The key identifies the tool in gateway calls and must be unique. Use 3–32 letters, numbers, underscores, or hyphens. Built-in tool keys cannot be changed.',
  '说明工具能做什么、适合何时使用，以及必要的输入。':
    'Describe what the tool does, when to use it, and what input it needs.',
  连接上游: 'Upstream connection',
  '内置工具由 Loadout 执行，无需连接地址。新增仅支持 echo（回声）和 time_now（当前时间）；接入自己的工具请选择 HTTP 服务。':
    'Built-in tools run in Loadout and need no endpoint. Only echo and time_now can be added as built-in tools. Choose HTTP service to connect your own tool.',
  '填写支持 MCP 的 HTTP 端点，不是普通 REST API。服务地址必须符合服务器允许的网络范围。':
    'Enter an HTTP endpoint that supports MCP, rather than a general REST API. Its address must be within the network ranges allowed by the server.',
  '进程在服务端运行。每个副本都必须安装相同程序，并配置 LOADOUT_STDIO_COMMANDS 允许名单；浏览器所在电脑不会启动进程。':
    'The process runs on the server. Every replica must have the same program installed and configure the LOADOUT_STDIO_COMMANDS allowlist. No process is started on the computer running your browser.',
  更新连接配置: 'Update connection settings',
  '已保存的连接与凭证继续使用，不会回显。需要修改时勾选更新，并完整填写新配置。':
    'The saved connection and credentials remain in use and are not displayed. To change them, select the update option and provide the complete replacement configuration.',
  填写方式: 'Input method',
  引导填写: 'Guided fields',
  '高级 JSON': 'Advanced JSON',
  '这里的示例需要替换为你实际使用的地址或命令；新配置会完整替换旧配置。':
    'Replace the example with your actual address or command. The new configuration replaces the existing configuration in full.',
  'MCP 服务地址': 'MCP service URL',
  'Bearer 令牌（可选）': 'Bearer token (optional)',
  '只填写令牌本身，系统会添加 Bearer 前缀；只使用运营方的上游凭证。':
    'Enter only the token; the Bearer prefix is added automatically. Use only upstream credentials managed by the operator.',
  '自定义请求头（可选）': 'Custom headers (optional)',
  '请求头（JSON）': 'Headers (JSON)',
  '名称和值都必须是字符串。填写 Bearer 令牌时，它会覆盖 Authorization 请求头。':
    'Header names and values must be strings. If a Bearer token is provided, it overrides the Authorization header.',
  命令别名: 'Command alias',
  '启动参数（JSON 数组，可选）': 'Arguments (JSON array, optional)',
  '环境变量（JSON，可选）': 'Environment variables (JSON, optional)',
  调用参数: 'Call parameters',
  'Schema 描述调用时允许的输入。type 固定为 object，properties 定义字段，required 列出必填字段。':
    'The schema describes the accepted input. Set type to object, define fields in properties, and list mandatory fields in required.',
  '这里保存工具的参数说明；HTTP 和本地进程的实际可调用工具由上游发现。保存后通过“查看参数”管理上游工具及参数覆盖。':
    'This stores the tool parameter description. Callable tools for HTTP services and local processes are discovered from the upstream service. After saving, use View parameters to manage upstream tools and parameter overrides.',
  参数填写示例: 'Parameter example',
  '下面是必填文本 query 的示例。description 告诉 AI 应该传什么；required 使用字段名数组，不能写成 true。':
    'This example defines a required text field named query. The description tells the AI what to pass. The required property must be an array of field names, not true.',
  要搜索的关键词: 'Keywords to search for',
  填入文本参数示例: 'Insert text parameter example',
  '填入示例会替换上方内容，保存工具后才生效。':
    'Inserting the example replaces the content above. Changes take effect after you save the tool.',
  额度与结算规则: 'Credits and settlement rules',
  '填 0 表示免费；只能填写非负整数。上游服务暴露多个工具时，每次工具调用按此额度计费。':
    'Enter 0 for free calls. Only non-negative integers are allowed. If the upstream service exposes multiple tools, each tool call uses this credit amount.',
  '内置工具使用协议结果结算，无需额外业务规则。':
    'Built-in tools settle based on the protocol result and need no additional business rules.',
  '正在保存…': 'Saving…',
  '参数 Schema 必须是 JSON 对象，请检查引号和逗号。':
    'The input schema must be a JSON object. Check the quotation marks and commas.',
  '参数 Schema 的 type 必须为 object。':
    'The input schema type must be object.',
  '连接配置必须是 JSON 对象，请检查引号和逗号。':
    'Connection settings must be a JSON object. Check the quotation marks and commas.',
  '请输入完整的 HTTP 或 HTTPS MCP 服务地址。':
    'Enter a complete HTTP or HTTPS MCP service URL.',
  '请求头必须是 JSON 对象，名称和值均为字符串。':
    'Headers must be a JSON object with string names and values.',
  '请填写服务器允许的命令别名。':
    'Enter a command alias allowed by the server.',
  '启动参数必须是 JSON 字符串数组，例如 ["--port","3000"]。':
    'Arguments must be a JSON array of strings, such as ["--port","3000"].',
  '环境变量必须是 JSON 对象，名称和值均为字符串。':
    'Environment variables must be a JSON object with string names and values.',
  '请检查填写内容。': 'Check the values you entered.',
  '请填写成功字段路径，例如 data.code（最多 64 字节）。':
    'Enter the success field path, such as data.code (up to 64 bytes).',
  '成功值必须是 JSON，例如 0、"ok"、true 或 [0,200]。':
    'The success value must be JSON, such as 0, "ok", true, or [0,200].',
  '请填写文本匹配表达式（最多 256 字节）。':
    'Enter a text matching expression (up to 256 bytes).',
  '结算策略必须是 JSON 对象，请检查引号和逗号。':
    'The settlement policy must be a JSON object. Check the quotation marks and commas.',
  '脚本不能为空，且不能超过 8 KiB。':
    'The script must not be empty or exceed 8 KiB.',
  '结算脚本语法有误，请检查后重试。':
    'The settlement script has a syntax error. Check it and try again.',
  '先预留每次调用额度，返回结果满足条件才扣费；上游报错、超时或规则未通过时退回额度。':
    'Credits are reserved before each call and charged only if the result meets the condition. Upstream errors, timeouts, or failed rules return the reserved credits.',
  扣费条件: 'Charging condition',
  协议成功即扣费: 'Charge on protocol success',
  'JSON 字段匹配': 'JSON field match',
  返回文本匹配: 'Response text match',
  'JavaScript 脚本': 'JavaScript script',
  '适合正常使用 MCP isError 标志的服务。HTTP 200 不代表业务成功；若返回 code 等业务码，请选 JSON 字段匹配。':
    'Use this when the service correctly sets the MCP isError flag. HTTP 200 does not guarantee business success. If the response includes a business code such as code, choose JSON field match.',
  成功字段路径: 'Success field path',
  '例如返回 {"data":{"code":0}}，路径填 data.code；只支持对象字段，用英文句点连接，不支持数组下标。':
    'For a response such as {"data":{"code":0}}, enter data.code. Only object fields separated by periods are supported; array indexes are not.',
  '成功值（JSON）': 'Success value (JSON)',
  '数字填 0，字符串填 "ok"，布尔值填 true；多个成功码填 [0,200]（最多 16 个）。字段缺失或返回内容不是 JSON 时退款。':
    'Use 0 for a number, "ok" for a string, or true for a boolean. For multiple success codes, use [0,200] (up to 16 values). Missing fields or non-JSON responses return the reserved credits.',
  文本匹配表达式: 'Text matching expression',
  '例如 ^OK 匹配以 OK 开头的返回文本。使用 Go 正则语法，不支持前后查找或反向引用；先用下方试算验证。':
    'For example, ^OK matches response text beginning with OK. Use Go regular expression syntax; lookarounds and backreferences are not supported. Verify the rule with the preview below.',
  判定脚本: 'Evaluation script',
  'result.text 是所有文本块拼接的字符串，result.isError 是协议错误标志。return 真值才扣费；异常或超过 200ms 即退款。沙箱不能联网、读文件或访问 Node.js，脚本最多 8 KiB。':
    'result.text contains all text blocks joined together, and result.isError is the protocol error flag. Return a truthy value to charge. Exceptions or execution exceeding 200ms return the reserved credits. The sandbox has no network, file, or Node.js access. Scripts must not exceed 8 KiB.',
  '结算策略（JSON，可选）': 'Settlement policy (JSON, optional)',
  '留空或 {} 恢复默认。三种规则互斥：content.path + equals、content.pattern、script。保存前可用下方真实引擎试算。':
    'Leave blank or enter {} to restore the default. Choose only one rule: content.path + equals, content.pattern, or script. Before saving, preview it below using the actual settlement engine.',
  '如何选择规则？': 'Which rule should I choose?',
  '上游已经通过 isError 正确标记失败时使用。':
    'Use when the upstream service correctly marks failures with isError.',
  '有明确业务成功码时使用，例如 code=0 或 status="ok"。':
    'Use when the response has an explicit business success code, such as code=0 or status="ok".',
  '返回自然语言或纯文本，成功结果有稳定特征时使用。':
    'Use for natural language or plain text responses with a consistent success pattern.',
  '需要同时检查多个字段时使用；优先采用前面的简单规则。':
    'Use when multiple fields must be checked together. Prefer the simpler rules above when possible.',
  保存前试算: 'Preview before saving',
  '粘贴一次真实上游返回的文本内容，用服务端相同引擎判定。不会调用上游、保存工具或扣额度。':
    'Paste the text from a real upstream response to evaluate it with the same server engine. This does not call the upstream service, save the tool, or charge credits.',
  上游返回文本: 'Upstream response text',
  '上游标记为错误（isError）': 'Upstream marked an error (isError)',
  '试算文本不能超过 64 KiB。': 'Preview text must not exceed 64 KiB.',
  '正在试算…': 'Evaluating…',
  试算规则: 'Preview rule',
  '满足条件，将扣除每次调用额度':
    'Condition met: credits per call will be charged',
  '不扣费，退回预留额度': 'No charge: reserved credits will be returned',
};
