INSERT INTO tools(key,name,description,kind,cost,input_schema) VALUES
 ('account_balance','Account balance','Returns the calling account username and current credit balance.','builtin',0,'{"type":"object","properties":{},"additionalProperties":false}'),
 ('account_usage','Account usage','Returns the calling account usage summary and recent call records.','builtin',0,'{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":50,"description":"Recent records to return"}},"additionalProperties":false}'),
 ('tools_catalog','Tools catalog','Returns the enabled gateway tools with description and cost per call.','builtin',0,'{"type":"object","properties":{},"additionalProperties":false}');
