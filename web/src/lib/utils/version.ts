/**
 * Version comparison utilities for Runner upgrade prompts.
 */

/**
 * Normalize a version string by stripping the "v" prefix.
 * e.g., "v1.2.3" -> "1.2.3", "1.2.3" -> "1.2.3"
 */
function normalizeVersion(version: string): string {
  return version.trim().replace(/^v/, "");
}

/**
 * Parse a semver string into [major, minor, patch] numbers.
 * Returns null if the version is not a valid semver.
 */
function parseSemver(version: string): [number, number, number] | null {
  const match = normalizeVersion(version).match(/^(\d+)\.(\d+)\.(\d+)/);
  if (!match) return null;
  return [parseInt(match[1], 10), parseInt(match[2], 10), parseInt(match[3], 10)];
}

/**
 * Check if a runner version is outdated compared to the latest version.
 *
 * Returns true if:
 * - Both versions are valid semver AND current < latest
 *
 * Returns false if:
 * - Either version is empty, undefined, "dev", or not a valid semver
 * - Versions are equal or current >= latest
 */
export function isVersionOutdated(
  current?: string | null,
  latest?: string | null
): boolean {
  if (!current || !latest) return false;

  // "dev" builds are never considered outdated (local development)
  if (current === "dev" || latest === "dev") return false;

  const currentParts = parseSemver(current);
  const latestParts = parseSemver(latest);

  if (!currentParts || !latestParts) return false;

  for (let i = 0; i < 3; i++) {
    if (currentParts[i] < latestParts[i]) return true;
    if (currentParts[i] > latestParts[i]) return false;
  }

  return false; // equal
}
