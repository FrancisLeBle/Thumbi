#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Git History Secret Audit Script (Phase 4)
# Scans the full Git history (all commits, branches, tags) for previously
# committed credentials, connection strings, private keys, or hardcoded tokens.
# ==============================================================================

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "ERROR: Run this script from inside the Git repository root."
  exit 1
fi

echo "==> Checking Git index for accidentally tracked build artifacts or .env files..."
git ls-files | grep -E '^(dist/|android/app/src/main/assets/public/|android/capacitor-cordova-android-plugins/|\.env($|\.))' || echo "OK: No build bundles or .env files currently tracked in index."

echo ""
echo "==> Running full Git history scan..."
if command -v gitleaks >/dev/null 2>&1; then
  gitleaks detect --source . --verbose --redact --log-opts="--all"
else
  echo "[WARN] 'gitleaks' CLI not found locally; running fallback git log regex audit."
  echo "       Install gitleaks for full entropy + rule scanning: https://github.com/gitleaks/gitleaks"
  git log --all -p -S "thumbi_secret" -S "thumbi-dev-only" -S "thumbi-vehicle-review-webhook-secret" \
    -S "POSTGRES_PASSWORD" -S "WEBHOOK_SECRET" -S "PRIVATE KEY" \
    -- . ':!*.lock' ':!go.sum' | grep -E "^(\+|-|commit )" | head -n 120 || true
fi

echo ""
echo "==> Targeted commit history check for known high-risk patterns:"
PATTERNS=(
  "thumbi_secret"
  "thumbi-dev-only-mock-webhook-secret-key-32b"
  "thumbi-vehicle-review-webhook-secret-key-32-chars"
  "postgres://[^[:space:]]+:[^[:space:]]+@"
  "BEGIN (RSA|EC|OPENSSH|PGP) PRIVATE KEY"
  "AIza[0-9A-Za-z_-]{35}"
  "sk_(live|test)_[0-9a-zA-Z]{24,}"
  "ghp_[0-9a-zA-Z]{36}"
)

for pattern in "${PATTERNS[@]}"; do
  matches=$(git log --all -G "${pattern}" --oneline | head -n 10 || true)
  if [[ -n "${matches}" ]]; then
    echo "[FINDING] Pattern '${pattern}' appeared in commits:"
    echo "${matches}"
  fi
done

echo ""
echo "Next steps if any real credential appeared in Git history:"
echo "  1. Rotate/revoke the exposed secret IMMEDIATELY at the provider (DB, webhook, OAuth, API key)."
echo "  2. Rewrite history with 'git filter-repo' (never rely solely on a new commit to delete secrets)."
echo "  3. Force-push cleaned refs and require collaborators to re-clone."
