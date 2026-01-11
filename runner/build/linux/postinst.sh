#!/bin/bash
# Post-installation script for AgentMesh Runner

set -e

# Create agentmesh user if it doesn't exist
if ! id "agentmesh" &>/dev/null; then
    useradd --system --no-create-home --shell /bin/false agentmesh
fi

# Create directories
mkdir -p /var/lib/agentmesh
mkdir -p /var/log/agentmesh
mkdir -p /etc/agentmesh

# Set permissions
chown -R agentmesh:agentmesh /var/lib/agentmesh
chown -R agentmesh:agentmesh /var/log/agentmesh
chmod 755 /var/lib/agentmesh
chmod 755 /var/log/agentmesh

# Reload systemd
systemctl daemon-reload

echo ""
echo "AgentMesh Runner has been installed."
echo ""
echo "Next steps:"
echo "  1. Register the runner:"
echo "     sudo -u agentmesh runner register --server <URL> --token <TOKEN>"
echo ""
echo "  2. Start the service:"
echo "     sudo systemctl start agentmesh-runner"
echo ""
echo "  3. Enable auto-start at boot:"
echo "     sudo systemctl enable agentmesh-runner"
echo ""
