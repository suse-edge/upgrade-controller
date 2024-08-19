#!/bin/sh

OS_UPGRADED_PLACEHOLDER_PATH="/etc/os-upgrade-successful"

if [ -f ${OS_UPGRADED_PLACEHOLDER_PATH} ]; then
    # Due to the nature of how SUC handles OS upgrades,
    # the OS upgrade pod will be restarted after an OS reboot.
    # Within the new Pod we only need to check whether the upgrade
    # has been done. This is done by checking for the '/run/os-upgrade-successful'
    # file which will only be present on the system if a successful upgrade
    # of the OS has taken place.
    echo "Upgrade has already been done. Exiting.."
    rm ${OS_UPGRADED_PLACEHOLDER_PATH}
    exit 0
fi

cleanupService(){
    rm ${1}
    systemctl daemon-reload
}

executeUpgrade(){
    # Common Platform Enumeration (CPE) coming from the release manifest
    RELEASE_CPE={{.CPEScheme}}
    # Common Platform Enumeration (CPE) that the system is currently running with
    CURRENT_CPE=`cat /etc/os-release | grep -w CPE_NAME | cut -d "=" -f 2 | tr -d '"'`

    # Determine whether architecture is supported
    SYSTEM_ARCH=`arch`
    IFS=' ' read -r -a SUPPORTED_ARCH_ARRAY <<< $(echo "{{.SupportedArchs}}" | tr -d '[]')

    found=false
    for arch in "${SUPPORTED_ARCH_ARRAY[@]}"; do
        if [ "${SYSTEM_ARCH}" == ${arch} ]; then
            found=true
            break
        fi
    done

    if [ ${found} == false ]; then
        echo "Operating system is running an unsupported architecture. System arch: ${SYSTEM_ARCH}. Supported archs: ${SUPPORTED_ARCH_ARRAY[*]}"
        exit 1
    fi

    # Lines that will be appended to the systemd.service 'ExecStartPre' configuration
    EXEC_START_PRE_LINES=""

    # Determine whether this is a package update or a migration
    if [ "${RELEASE_CPE}" == "${CURRENT_CPE}" ]; then
        # Package update if both CPEs are the same
        EXEC_START="/usr/sbin/transactional-update cleanup up"
        SERVICE_NAME="os-pkg-update.service"
    else
        # Migration if the CPEs are different
        EXEC_START_PRE_PKG_UPGRADE="ExecStartPre=/usr/sbin/transactional-update cleanup up"
        EXEC_START_PRE_RPM_IMPORT="ExecStartPre=/usr/sbin/transactional-update --continue run rpm --import {{.RepoGPGKey}}"
        EXEC_START_PRE_LINES=$(echo -e "${EXEC_START_PRE_PKG_UPGRADE}\n${EXEC_START_PRE_RPM_IMPORT}")

        EXEC_START="/usr/sbin/transactional-update --continue run zypper migration --non-interactive --product {{.ZypperID}}/{{.Version}}/${SYSTEM_ARCH} --root /"
        SERVICE_NAME="os-migration.service"
    fi

    UPDATE_SERVICE_PATH=/etc/systemd/system/${SERVICE_NAME}

    # Make sure that even after a non-zero exit of the script
    # we will do a cleanup of the service
    trap "cleanupService ${UPDATE_SERVICE_PATH}" EXIT

    echo "Creating ${SERVICE_NAME}..."
    cat <<EOF > ${UPDATE_SERVICE_PATH}
[Unit]
Description=SUSE Edge Upgrade Service
ConditionACPower=true
Wants=network.target
After=network.target

[Service]
Type=oneshot
${EXEC_START_PRE_LINES:+$EXEC_START_PRE_LINES
}ExecStart=${EXEC_START}
IOSchedulingClass=best-effort
IOSchedulingPriority=7
EOF

    echo "Starting ${SERVICE_NAME}..."
    systemctl start ${SERVICE_NAME} &

    BACKGROUND_PROC_PID=$!
    tail --pid ${BACKGROUND_PROC_PID} -f /var/log/transactional-update.log

    # Waits for the background process with pid to finish and propagates its exit code to '$?'
    wait ${BACKGROUND_PROC_PID}

    # Get exit code of backgroup process 
    BACKGROUND_PROC_EXIT=$?
    if [ ${BACKGROUND_PROC_EXIT} -ne 0 ]; then
        exit ${BACKGROUND_PROC_EXIT}
    fi

    # Check if reboot is needed.
    # Will only be needed when transactional-update has successfully
    # done any package upgrades/updates.
    if [ -f /run/reboot-needed ]; then
        # Create a placeholder indicating that the os upgrade
        # has finished succesfully
        touch ${OS_UPGRADED_PLACEHOLDER_PATH}
        /usr/sbin/reboot
    fi
}

executeUpgrade
