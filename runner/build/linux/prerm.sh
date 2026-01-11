#!/bin/bash
# Pre-removal script for AgentMesh Runner

set -e

# Stop the service if running
if systemctl is-active --quiet agentmesh-runner; then
    systemctl stop agentmesh-runner
fi

# Disable the service
if systemctl is-enabled --quiet agentmesh-runner 2>/dev/null; then
    systemctl disable agentmesh-runner
fi

echo "AgentMesh Runner service stopped and disabled."
