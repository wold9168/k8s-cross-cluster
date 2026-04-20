"""Headscale REST API helpers used during installation."""

from __future__ import annotations

import json
from dataclasses import dataclass
from urllib import error, parse, request


@dataclass(frozen=True)
class HeadscaleNode:
    """Subset of Headscale node fields used for duplicate checks."""

    name: str | None = None
    given_name: str | None = None


class HeadscaleClient:
    """Minimal Headscale REST API client for node lookups."""

    def __init__(self, base_url: str, api_key: str, timeout: float = 10.0):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout

    def _build_url(self, path: str) -> str:
        api_base = self.base_url
        if not api_base.endswith("/api/v1"):
            api_base = f"{api_base}/api/v1"
        return f"{api_base}{path}"

    def _request_json(self, path: str) -> dict:
        req = request.Request(
            self._build_url(path),
            headers={
                "Authorization": f"Bearer {self.api_key}",
                "Accept": "application/json",
            },
        )

        try:
            with request.urlopen(req, timeout=self.timeout) as response:
                payload = response.read().decode("utf-8")
        except error.HTTPError as exc:
            raise RuntimeError(
                f"Headscale API request failed with status {exc.code}: {exc.reason}"
            ) from exc
        except error.URLError as exc:
            raise RuntimeError(f"Unable to reach Headscale API: {exc.reason}") from exc

        try:
            data = json.loads(payload)
        except json.JSONDecodeError as exc:
            raise RuntimeError("Headscale API returned invalid JSON") from exc

        if not isinstance(data, dict):
            raise RuntimeError("Headscale API returned an unexpected response")
        return data

    def list_nodes(self) -> list[HeadscaleNode]:
        """Return all nodes known to Headscale."""

        response = self._request_json("/node")
        raw_nodes = response.get("nodes", [])
        if not isinstance(raw_nodes, list):
            raise RuntimeError("Headscale API returned an unexpected nodes payload")

        nodes = []
        for raw_node in raw_nodes:
            if not isinstance(raw_node, dict):
                continue
            nodes.append(
                HeadscaleNode(
                    name=raw_node.get("name"),
                    given_name=raw_node.get("givenName") or raw_node.get("given_name"),
                )
            )
        return nodes

    def node_exists(self, hostname: str) -> bool:
        """Return True if Headscale already has a node with this hostname."""

        target = hostname.casefold()
        for node in self.list_nodes():
            candidate_names = [node.name, node.given_name]
            if any(name and name.casefold() == target for name in candidate_names):
                return True
        return False
