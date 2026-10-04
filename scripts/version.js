// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

"use strict";

const SEMVER_RE =
  /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;

function parseVersionOutput(output) {
  const match = String(output || "").match(
    /v(\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?)/
  );
  return match ? match[1] : null;
}

function semverGt(a, b) {
  if (!SEMVER_RE.test(a) || !SEMVER_RE.test(b)) return false;

  const aWithoutBuild = a.replace(/\+.*$/, "");
  const bWithoutBuild = b.replace(/\+.*$/, "");
  const pa = aWithoutBuild.replace(/-.*$/, "").split(".").map(Number);
  const pb = bWithoutBuild.replace(/-.*$/, "").split(".").map(Number);
  for (let i = 0; i < 3; i++) {
    if (pa[i] > pb[i]) return true;
    if (pa[i] < pb[i]) return false;
  }
  const aPre = aWithoutBuild.includes("-");
  const bPre = bWithoutBuild.includes("-");
  if (bPre && !aPre) return true;
  if (!aPre || !bPre) return false;
  const ia = aWithoutBuild.slice(aWithoutBuild.indexOf("-") + 1).split(".");
  const ib = bWithoutBuild.slice(bWithoutBuild.indexOf("-") + 1).split(".");
  for (let i = 0; i < Math.min(ia.length, ib.length); i++) {
    if (ia[i] === ib[i]) continue;
    const na = /^\d+$/.test(ia[i]);
    const nb = /^\d+$/.test(ib[i]);
    if (na && nb) return Number(ia[i]) > Number(ib[i]);
    // Numeric identifiers have lower precedence than alphanumeric ones.
    if (na !== nb) return nb;
    return ia[i] > ib[i];
  }
  return ia.length > ib.length;
}

function shouldShowUpdateHint(hintVersion, installedVersion) {
  if (!SEMVER_RE.test(hintVersion) || !SEMVER_RE.test(installedVersion)) {
    return false;
  }
  return semverGt(hintVersion, installedVersion);
}

module.exports = {
  SEMVER_RE,
  parseVersionOutput,
  semverGt,
  shouldShowUpdateHint,
};
