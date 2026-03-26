UPDATE agent_types SET
    config_schema = '{
        "fields": [
            {
                "name": "mcp_enabled",
                "type": "boolean",
                "default": true
            },
            {
                "name": "skip_permissions",
                "type": "boolean",
                "default": false
            },
            {
                "name": "models",
                "type": "model_list"
            },
            {
                "name": "model",
                "type": "select",
                "default": ""
            }
        ]
    }'::jsonb,
    command_template = '{
        "args": [
            {
                "condition": {"field": "model", "operator": "not_empty"},
                "args": ["--model", "{{.config.model}}"]
            }
        ]
    }'::jsonb
WHERE slug = 'opencode';
