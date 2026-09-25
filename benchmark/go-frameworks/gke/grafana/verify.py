"""Runs every dashboard panel query through Grafana's data source proxy (used by deploy.sh)."""
import base64
import json
import os
import sys
import urllib.parse
import urllib.request

BASE = "http://localhost:3000"
AUTH = "Basic " + base64.b64encode(f"admin:{os.environ['GRAFANA_PW']}".encode()).decode()


def get(path, **params):
    url = f"{BASE}{path}?{urllib.parse.urlencode(params)}" if params else BASE + path
    req = urllib.request.Request(url, headers={"Authorization": AUTH})
    return json.load(urllib.request.urlopen(req))


at = sys.argv[1]
dash = get("/api/dashboards/uid/go-frameworks")["dashboard"]
print(f"dashboard: {dash['title']}, {len(dash['panels'])} panels")
for p in dash["panels"]:
    res = get("/api/datasources/proxy/uid/gmp/api/v1/query", query=p["targets"][0]["expr"], time=at)
    series = res["data"]["result"]
    values = ", ".join(f"{s['metric'].get('framework')}={float(s['value'][1]):.4g}" for s in series)
    print(f"{p['title']}: {values or 'NO DATA'}")
