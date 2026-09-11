-- 网关供给型市场组件：无公共端点，指向池内工具（installed_tool_id），经 bridge 计量使用。
ALTER TABLE marketplace_items DROP CONSTRAINT marketplace_items_transport_check;
ALTER TABLE marketplace_items ADD CONSTRAINT marketplace_items_transport_check CHECK(transport IN ('http','stdio','unknown','gateway'));
