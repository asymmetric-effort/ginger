#!/usr/bin/env python3
"""PyPI OIDC publish: mint GitHub OIDC token, exchange with PyPI, upload."""

import glob
import json
import os
import subprocess
import sys
import urllib.request


def main() -> int:
    # Step 1: Get GitHub OIDC token
    token_url = os.environ.get("ACTIONS_ID_TOKEN_REQUEST_URL")
    token_auth = os.environ.get("ACTIONS_ID_TOKEN_REQUEST_TOKEN")

    if not token_url or not token_auth:
        print("ERROR: ACTIONS_ID_TOKEN_REQUEST_URL/TOKEN not set.")
        print("Ensure the job has 'permissions: id-token: write'")
        return 1

    # Request OIDC token with PyPI audience
    url = f"{token_url}&audience=pypi"
    req = urllib.request.Request(url, headers={"Authorization": f"bearer {token_auth}"})
    try:
        with urllib.request.urlopen(req) as resp:
            oidc_token = json.loads(resp.read())["value"]
    except Exception as e:
        print(f"ERROR: Failed to mint OIDC token: {e}")
        return 1

    print("OIDC token minted successfully")

    # Step 2: Exchange OIDC token with PyPI for short-lived upload token
    exchange_url = "https://pypi.org/_/oidc/mint-token"
    payload = json.dumps({"token": oidc_token}).encode()
    req = urllib.request.Request(
        exchange_url,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req) as resp:
            resp_data = json.loads(resp.read())
            pypi_token = resp_data["token"]
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        print(f"ERROR: PyPI token exchange failed: {e.code} {body}")
        return 1
    except Exception as e:
        print(f"ERROR: PyPI token exchange failed: {e}")
        return 1

    print("PyPI upload token obtained via OIDC exchange")

    # Step 3: Find distribution files
    dist_dir = os.path.join("sdk", "python", "dist")
    dist_files = glob.glob(os.path.join(dist_dir, "*"))
    if not dist_files:
        print(f"ERROR: No distribution files found in {dist_dir}")
        return 1

    print(f"Uploading {len(dist_files)} file(s): {[os.path.basename(f) for f in dist_files]}")

    # Step 4: Upload with twine
    env = os.environ.copy()
    env["TWINE_USERNAME"] = "__token__"
    env["TWINE_PASSWORD"] = pypi_token

    cmd = ["twine", "upload", "--non-interactive"] + dist_files
    result = subprocess.run(cmd, env=env)
    return result.returncode


if __name__ == "__main__":
    sys.exit(main())
