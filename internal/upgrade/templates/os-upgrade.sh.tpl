#!/bin/sh

# Common Platform Enumeration (CPE) comming from the release manifest
RELEASE_CPE={{.CPEScheme}}
# Common Platform Enumeration (CPE) that the system is currently running with
CURRENT_CPE=`cat /etc/os-release | grep -w CPE_NAME | cut -d "=" -f 2 | tr -d '"'`

# Determine whether architecture is supported
SYSTEM_ARCH=`arch`
IFS=' ' read -r -a SUPPORTED_ARCH_ARRAY <<< $(echo "{{.SupportedArchs}}" | tr -d '[]')

found=false
for arch in "${SUPPORTED_ARCH_ARRAY[@]}"; do
    echo "$arch"
    if [ "${SYSTEM_ARCH}" == ${arch} ]; then
        found=true
        break
    fi
done

if [ ${found} == false ]; then
    echo "Operating system is running an unsupported architecture. System arch: ${SYSTEM_ARCH}. Supported archs: ${SUPPORTED_ARCH_ARRAY[*]}"
    exit 1
fi

# Determine whether this is a package update or a migration
if [ "${RELEASE_CPE}" == "${CURRENT_CPE}" ]; then
    # Package update if both CPEs are the same
    EXEC_START_PRE=""
    EXEC_START="/usr/sbin/transactional-update cleanup up"
    SERVICE_NAME="os-pkg-update.service"
else
    # Migration if the CPEs are different
    EXEC_START_PRE="/usr/sbin/transactional-update run rpm --import {{.RepoGPGKey}}"
    EXEC_START="/usr/sbin/transactional-update --continue run zypper migration --non-interactive --product {{.ZypperID}}/{{.Version}}/${SYSTEM_ARCH} --root /"
    SERVICE_NAME="os-migration.service"
fi

UPDATE_SERVICE_PATH=/etc/systemd/system/${SERVICE_NAME}

echo "Creating ${SERVICE_NAME}..."
cat <<EOF > ${UPDATE_SERVICE_PATH}
[Unit]
Description=SUSE Edge Upgrade Service
ConditionACPower=true
Wants=network.target
After=network.target

[Service]
Type=oneshot
ExecStartPre=${EXEC_START_PRE}
ExecStart=${EXEC_START}
ExecStartPost=-/bin/bash -c '[ -f /run/reboot-needed ] && shutdown -r +1'
IOSchedulingClass=best-effort
IOSchedulingPriority=7
EOF

echo "Starting ${SERVICE_NAME}..."
systemctl start ${SERVICE_NAME} &
tail --pid $! -f cat /var/log/transactional-update.log

echo "Cleaning up..."
# Remove service after it has finished its work
rm ${UPDATE_SERVICE_PATH}
systemctl daemon-reload
