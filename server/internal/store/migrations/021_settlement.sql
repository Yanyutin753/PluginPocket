-- 结算策略：默认空对象 = 仅协议层 isError 判失败（现状，失败已退款）。
-- {"content":{"path":"code","equals":0}} —— 业务体 JSON 路径判等；
-- {"content":{"pattern":"^OK"}} —— 拼接文本正则匹配。
ALTER TABLE tools ADD COLUMN settlement jsonb NOT NULL DEFAULT '{}'::jsonb;
