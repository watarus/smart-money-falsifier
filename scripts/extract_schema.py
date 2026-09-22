#!/usr/bin/env python3
"""Extract just the request schema + path from a Nansen docs .md page.

Each docs page embeds a full OpenAPI document in a ```json fence. Printing that
whole thing costs ~20k chars, so pull out only what a client needs:
the POST path and the flattened request-body properties.
"""
import json
import re
import sys


def load_spec(path):
    text = open(path).read()
    m = re.search(r"```json\n(\{.*?\})\n```", text, re.S)
    if not m:
        sys.exit(f"{path}: no json fence")
    return json.loads(m.group(1))


def resolve(schemas, node, depth=0):
    """Render a schema node as a short type string, following $ref one level."""
    if "$ref" in node:
        name = node["$ref"].rsplit("/", 1)[-1]
        target = schemas.get(name, {})
        if "enum" in target:
            return "|".join(map(str, target["enum"]))
        if depth < 1 and target.get("type") == "object":
            return f"{name}{{{', '.join(target.get('properties', {}))}}}"
        return name
    if "enum" in node:
        return "|".join(map(str, node["enum"]))
    if "anyOf" in node:
        return "/".join(resolve(schemas, a, depth + 1) for a in node["anyOf"])
    t = node.get("type", "?")
    if t == "array":
        return f"[{resolve(schemas, node.get('items', {}), depth + 1)}]"
    return t


def main():
    for path in sys.argv[1:]:
        spec = load_spec(path)
        schemas = spec["components"]["schemas"]
        for route, methods in spec.get("paths", {}).items():
            for method, op in methods.items():
                print(f"\n=== {method.upper()} {route}  ({path})")
                body = op.get("requestBody", {}).get("content", {})
                ref = body.get("application/json", {}).get("schema", {}).get("$ref")
                if not ref:
                    print("  (no json body)")
                    continue
                req = schemas[ref.rsplit("/", 1)[-1]]
                required = set(req.get("required", []))
                for field, node in req.get("properties", {}).items():
                    mark = "*" if field in required else " "
                    desc = node.get("description", "")[:60]
                    print(f"  {mark} {field}: {resolve(schemas, node)}  {desc}")


if __name__ == "__main__":
    main()
