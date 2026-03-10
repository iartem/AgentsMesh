/**
 * Utility functions for Pod display
 */

interface PodDisplayInfo {
  pod_key: string;
  title?: string | null;
  ticket?: {
    slug?: string;
    title?: string;
  };
  agent_type?: {
    name?: string;
  };
}

/**
 * Get the display name for a Pod.
 *
 * Priority:
 * 1. Ticket title + agent type name (if associated with a ticket)
 * 2. OSC title (set by terminal applications like Claude Code)
 * 3. Ticket slug fallback
 * 4. Agent type name + truncated pod_key
 *
 * @param pod - Pod data with optional title and ticket
 * @param maxLength - Maximum length before truncation (default: 20)
 * @returns Display name string
 */
export function getPodDisplayName(
  pod: PodDisplayInfo,
  maxLength: number = 20
): string {
  // Priority 1: Ticket title + agent type name
  // This takes precedence over OSC title because agents (e.g., Claude Code)
  // overwrite the terminal title with their own name, losing the ticket context.
  if (pod.ticket?.title) {
    const agentSuffix = pod.agent_type?.name ? ` (${pod.agent_type.name})` : "";
    const displayName = pod.ticket.title + agentSuffix;
    if (displayName.length > maxLength) {
      return displayName.substring(0, maxLength - 3) + "...";
    }
    return displayName;
  }

  // Priority 2: OSC title (set by terminal applications)
  if (pod.title) {
    if (pod.title.length > maxLength) {
      return pod.title.substring(0, maxLength - 3) + "...";
    }
    return pod.title;
  }

  // Priority 3: Ticket slug fallback
  if (pod.ticket?.slug) {
    return pod.ticket.slug;
  }

  // Priority 4: Agent type + truncated pod_key
  const keyPrefix = pod.pod_key.substring(0, 8);
  if (pod.agent_type?.name) {
    return `${pod.agent_type.name} (${keyPrefix})`;
  }

  // Fallback: just the truncated pod_key
  return `${keyPrefix}...`;
}
