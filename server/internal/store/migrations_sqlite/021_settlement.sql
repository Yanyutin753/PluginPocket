-- 结算策略：默认空对象 = 仅协议层 isError 判失败；content/script 见 PG 轨道注释。
ALTER TABLE tools ADD COLUMN settlement TEXT NOT NULL DEFAULT '{}';
