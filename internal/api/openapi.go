package api

import "github.com/gofiber/fiber/v2"

const openAPISpecJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "TraceMind API",
    "version": "1.0.0",
    "description": "Typical workflow:\n\n1. Optional setup:\n   - Configure an environment payload allow-list with POST /api/payload-filters/{environment}.\n   - For deterministic rules, create a rule with POST /api/analysis-rules, then add patterns with POST /api/analysis-rule-patterns using the returned rule ID.\n   Both setup steps are optional; neither is required to ingest signals.\n2. Submit signals with POST /api/ingest and keep the returned ingestionId.\n3. Follow processing status with GET /api/ingest/{id}/events; this is a server-sent event stream.\n4. Review results with GET /api/incidents, then GET /api/incidents/{id}.\n5. For queue troubleshooting, check GET /api/health/ingestion separately.\n\nThe endpoint reference below is sorted alphabetically for lookup."
  },
  "servers": [
    {
      "url": "/",
      "description": "Current server"
    }
  ],
  "paths": {
    "/": {
      "get": {
        "summary": "Root health message",
        "description": "Check whether the HTTP application is running. For queue availability and queue metrics, use GET /api/health/ingestion.",
        "tags": ["system"],
        "responses": {
          "200": {
            "description": "Application status",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "status": {"type": "string"},
                    "message": {"type": "string"}
                  },
                  "required": ["status", "message"]
                },
                "example": {
                  "status": "ok",
                  "message": "TraceMind Fiber app is running"
                }
              }
            }
          }
        }
      }
    },
    "/api/analysis-rule-patterns": {
      "post": {
        "summary": "Create analysis rule pattern",
        "description": "Create a pattern for an existing analysis rule. Use the rule ID returned by POST /api/analysis-rules as ruleId. The response provides the pattern ID; an identical duplicate returns the existing ID with created=false.",
        "tags": ["analysis-rule-patterns"],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AnalysisRulePatternRequest"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "Pattern created",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AnalysisRulePatternUpsertResponse"}
              }
            }
          },
          "200": {
            "description": "Duplicate payload resolved to existing pattern",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AnalysisRulePatternUpsertResponse"}
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    },
    "/api/analysis-rule-patterns/{id}": {
      "delete": {
        "summary": "Delete analysis rule pattern",
        "description": "Delete the pattern identified by the path ID. This does not delete its associated analysis rule.",
        "tags": ["analysis-rule-patterns"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "Pattern deleted",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/SuccessByIDResponse"}
              }
            }
          },
          "400": {
            "description": "Invalid path parameter",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "404": {
            "description": "Pattern not found",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      },
      "put": {
        "summary": "Update analysis rule pattern",
        "description": "Update the pattern identified by the path ID using the supplied pattern fields. The response confirms the updated pattern ID.",
        "tags": ["analysis-rule-patterns"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AnalysisRulePatternRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Pattern updated",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/SuccessByIDResponse"}
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "404": {
            "description": "Pattern not found",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    },
    "/api/analysis-rules": {
      "post": {
        "summary": "Create analysis rule",
        "description": "Create a rule that defines analysis behavior. Save the returned ID and use it as ruleId when creating one or more patterns. An identical duplicate returns the existing ID with created=false.",
        "tags": ["analysis-rules"],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AnalysisRuleRequest"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "Rule created",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AnalysisRuleUpsertResponse"}
              }
            }
          },
          "200": {
            "description": "Duplicate payload resolved to existing rule",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/AnalysisRuleUpsertResponse"}
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    },
    "/api/analysis-rules/{id}": {
      "delete": {
        "summary": "Delete analysis rule",
        "description": "Delete the analysis rule identified by the path ID. The response confirms the affected rule ID.",
        "tags": ["analysis-rules"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "Rule deleted",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/SuccessByIDResponse"}
              }
            }
          },
          "400": {
            "description": "Invalid path parameter",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "404": {
            "description": "Rule not found",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      },
      "put": {
        "summary": "Update analysis rule",
        "description": "Update the analysis rule identified by the path ID using the supplied rule fields. The response confirms the updated rule ID.",
        "tags": ["analysis-rules"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/AnalysisRuleRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Rule updated",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/SuccessByIDResponse"}
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "404": {
            "description": "Rule not found",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    },
    "/api/health/ingestion": {
      "get": {
        "summary": "Get ingestion queue health",
        "description": "Check queue availability and current queue counts. This diagnostic endpoint is independent of the ingest-to-incident workflow.",
        "tags": ["health"],
        "responses": {
          "200": {
            "description": "Queue health status",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/HealthResponse"}
              }
            }
          },
          "503": {
            "description": "Queue unavailable",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    },
    "/api/incidents": {
      "get": {
        "summary": "List incidents",
        "description": "List incidents produced by signal analysis. Use an incident ID from this response with GET /api/incidents/{id} to inspect its details.",
        "tags": ["incidents"],
        "responses": {
          "200": {
            "description": "Incident list",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/IncidentsListResponse"}
              }
            }
          }
        }
      }
    },
    "/api/incidents/{id}": {
      "get": {
        "summary": "Get incident by id",
        "description": "Retrieve one incident by its ID, typically obtained from GET /api/incidents.",
        "tags": ["incidents"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "Incident details",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/Incident"}
              }
            }
          },
          "400": {
            "description": "Missing id",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "404": {
            "description": "Incident not found",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    },
    "/api/ingest": {
      "post": {
        "summary": "Ingest one or more signals",
        "description": "Submit a non-empty batch of signals for asynchronous processing. A successful response includes ingestionId and accepted/rejected counts; retain ingestionId for GET /api/ingest/{id}/events. A 200 response can include rejected signals, so inspect the counts and errors.",
        "tags": ["ingest"],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/IngestRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Ingest result",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/IngestResponse"}
              }
            }
          },
          "400": {
            "description": "Invalid request body",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    },
    "/api/ingest/{id}/events": {
      "get": {
        "summary": "Stream ingestion status events (SSE)",
        "description": "Connect using the ingestionId returned by POST /api/ingest. The response is a text/event-stream, not JSON, and publishes status updates until processing reaches a terminal completed or failed state.",
        "tags": ["ingest"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "Server-sent event stream",
            "content": {
              "text/event-stream": {
                "schema": {
                  "type": "string"
                },
                "example": "id: ingestion-id\\nstatus: pending\\n\\n"
              }
            }
          },
          "400": {
            "description": "Missing ingestion id",
            "content": {
              "text/plain": {
                "schema": {
                  "type": "string"
                }
              }
            }
          },
          "404": {
            "description": "Ingestion not found",
            "content": {
              "text/plain": {
                "schema": {
                  "type": "string"
                }
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "text/plain": {
                "schema": {
                  "type": "string"
                }
              }
            }
          }
        }
      }
    },
    "/api/payload-filters/{environment}": {
      "delete": {
        "summary": "Delete keys from payload allow-list",
        "description": "Remove the listed payload keys from the allow-list for the specified environment. This configuration is optional and does not need to be changed before ingesting signals.",
        "tags": ["payload-filters"],
        "parameters": [
          {
            "name": "environment",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/PayloadFilterRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Payload allow-list updated",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/PayloadFilterResponse"}
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      },
      "post": {
        "summary": "Save payload allow-list",
        "description": "Save the payload keys permitted for the specified environment. This configuration is optional; when used, configure it before sending signals whose payload fields should be retained.",
        "tags": ["payload-filters"],
        "parameters": [
          {
            "name": "environment",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/PayloadFilterRequest"}
            }
          }
        },
        "responses": {
          "200": {
            "description": "Payload allow-list updated",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/PayloadFilterResponse"}
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          },
          "500": {
            "description": "Internal server error",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/ErrorResponse"}
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "ErrorResponse": {
        "type": "object",
        "properties": {
          "error": {"type": "string"}
        },
        "required": ["error"]
      },
      "HealthResponse": {
        "type": "object",
        "properties": {
          "status": {"type": "string"},
          "available": {"type": "integer"},
          "inFlight": {"type": "integer"},
          "delayed": {"type": "integer"}
        },
        "required": ["status", "available", "inFlight", "delayed"]
      },
      "SignalInput": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "eventType": {"type": "string", "enum": ["log", "deployment", "database", "queue", "health"]},
          "source": {"type": "string"},
          "environment": {"type": "string"},
          "timestamp": {"type": "string", "format": "date-time"},
          "severity": {"type": "integer", "minimum": 0, "maximum": 5},
          "message": {"type": "string"},
          "payload": {"type": "object", "additionalProperties": true},
          "metadata": {"type": "object", "additionalProperties": {"type": "string"}}
        },
        "required": ["eventType", "source", "environment", "severity"]
      },
      "IngestRequest": {
        "type": "object",
        "properties": {
          "sourceContext": {"type": "string"},
          "signals": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/SignalInput"}
          }
        },
        "required": ["signals"]
      },
      "IngestResponse": {
        "type": "object",
        "properties": {
          "ingestionId": {"type": "string"},
          "acceptedCount": {"type": "integer"},
          "duplicateCount": {"type": "integer"},
          "rejectedCount": {"type": "integer"},
          "errors": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["ingestionId", "acceptedCount", "rejectedCount"]
      },
      "Incident": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "title": {"type": "string"},
          "status": {"type": "string"},
          "severity": {"type": "integer"},
          "impactedServices": {"type": "array", "items": {"type": "string"}},
          "environments": {"type": "array", "items": {"type": "string"}},
          "signalIds": {"type": "array", "items": {"type": "string"}},
          "analysisSummary": {"type": "string"},
          "recommendations": {"type": "array", "items": {"type": "string"}},
          "createdAt": {"type": "string", "format": "date-time"},
          "updatedAt": {"type": "string", "format": "date-time"}
        },
        "required": ["id", "title", "status", "severity", "createdAt", "updatedAt"]
      },
      "IncidentsListResponse": {
        "type": "object",
        "properties": {
          "incidents": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/Incident"}
          }
        },
        "required": ["incidents"]
      },
      "PayloadFilterRequest": {
        "type": "object",
        "properties": {
          "payloads": {
            "type": "array",
            "items": {"type": "string"}
          }
        },
        "required": ["payloads"]
      },
      "PayloadFilterResponse": {
        "type": "object",
        "properties": {
          "status": {"type": "string"},
          "message": {"type": "string"},
          "environment": {"type": "string"},
          "count": {"type": "integer"}
        },
        "required": ["status", "message", "environment", "count"]
      },
      "AnalysisRuleRequest": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "name": {"type": "string"},
          "description": {"type": "string"},
          "confidence": {"type": "number", "format": "double", "minimum": 0, "maximum": 1},
          "priority": {"type": "integer", "minimum": 0, "maximum": 100},
          "enabled": {"type": "boolean"},
          "matchType": {"type": "string", "enum": ["single", "correlation"]},
          "hypothesisTemplate": {"type": "string"},
          "recommendations": {"type": "array", "items": {"type": "string"}},
          "version": {"type": "integer", "minimum": 1}
        },
        "required": ["name", "confidence", "matchType", "hypothesisTemplate"]
      },
      "AnalysisRuleUpsertResponse": {
        "type": "object",
        "properties": {
          "status": {"type": "string"},
          "id": {"type": "string"},
          "created": {"type": "boolean"}
        },
        "required": ["status", "id", "created"]
      },
      "PayloadCondition": {
        "type": "object",
        "properties": {
          "field": {"type": "string"},
          "operator": {"type": "string"},
          "value": {}
        },
        "required": ["field", "operator", "value"]
      },
      "AnalysisRulePatternRequest": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "ruleId": {"type": "string"},
          "eventType": {"type": "string"},
          "source": {"type": "string"},
          "environment": {"type": "string"},
          "severityMin": {"type": "integer", "minimum": 0, "maximum": 100},
          "messageMatchType": {"type": "string", "enum": ["exact", "contains", "regex"]},
          "messagePattern": {"type": "string"},
          "payloadConditions": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/PayloadCondition"}
          },
          "variableMappings": {
            "type": "object",
            "additionalProperties": {"type": "string"}
          }
        },
        "required": ["ruleId", "eventType", "source", "environment", "severityMin", "messageMatchType", "messagePattern"]
      },
      "AnalysisRulePatternUpsertResponse": {
        "type": "object",
        "properties": {
          "status": {"type": "string"},
          "id": {"type": "string"},
          "created": {"type": "boolean"}
        },
        "required": ["status", "id", "created"]
      },
      "SuccessByIDResponse": {
        "type": "object",
        "properties": {
          "status": {"type": "string"},
          "id": {"type": "string"}
        },
        "required": ["status", "id"]
      }
    }
  }
}`

const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>TraceMind API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body {
      margin: 0;
      background: #f4f7fb;
      font-family: "Segoe UI", Tahoma, Geneva, Verdana, sans-serif;
    }
    .header {
      padding: 16px 20px;
      background: linear-gradient(90deg, #0f172a, #1e293b);
      color: #e2e8f0;
    }
    .header h1 {
      margin: 0;
      font-size: 20px;
    }
    .header p {
      margin: 6px 0 0;
      font-size: 13px;
      opacity: 0.9;
    }
  </style>
</head>
<body>
  <div class="header">
    <h1>TraceMind OpenAPI / Swagger</h1>
    <p>Endpoints are sorted A-Z. Spec source: /api/openapi.json</p>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: '/api/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: 'BaseLayout',
        operationsSorter: 'alpha',
        tagsSorter: 'alpha'
      });
    };
  </script>
</body>
</html>`

func OpenAPISpecHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.SendString(openAPISpecJSON)
	}
}

func SwaggerUIHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.SendString(swaggerUIHTML)
	}
}
