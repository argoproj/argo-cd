local health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local numDegraded = 0
    local numPending = 0
    local msg = ""

    -- Check if this is a manual approval scenario where InstallPlanPending is expected
    -- and the operator is already installed (upgrade pending, not initial install)
    local isManualApprovalPending = false
    if obj.spec ~= nil and obj.spec.installPlanApproval == "Manual" then
      for _, condition in pairs(obj.status.conditions) do
        if condition.type == "InstallPlanPending" and condition.status == "True" and condition.reason == "RequiresApproval" then
          -- Only treat as expected healthy state if the operator is already installed
          -- (installedCSV is present), meaning this is an upgrade pending approval
          if obj.status.installedCSV ~= nil then
            isManualApprovalPending = true
          end
          break
        end
      end
    end

    for i, condition in pairs(obj.status.conditions) do
      -- Skip InstallPlanPending condition when manual approval is pending (expected behavior)
      if isManualApprovalPending and condition.type == "InstallPlanPending" then
        -- Do not include in message or count as pending
      else
        msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. "\n"
        if condition.type == "InstallPlanPending" and condition.status == "True" then
          numPending = numPending + 1
        elseif (condition.type == "InstallPlanMissing" and condition.reason ~= "ReferencedInstallPlanNotFound") then
          numDegraded = numDegraded + 1
        elseif (condition.type == "CatalogSourcesUnhealthy" or condition.type == "InstallPlanFailed" or condition.type == "ResolutionFailed") and condition.status == "True" then
          numDegraded = numDegraded + 1
        end
      end
    end

    -- Available states: undef/nil, UpgradeAvailable, UpgradePending, UpgradeFailed, AtLatestKnown
    -- Source: https://github.com/openshift/operator-framework-olm/blob/5e2c73b7663d0122c9dc3e59ea39e515a31e2719/staging/api/pkg/operators/v1alpha1/subscription_types.go#L17-L23
    if obj.status.state == nil  then
      numPending = numPending + 1
      msg = msg .. ".status.state not yet known\n"
    elseif obj.status.state == "" or obj.status.state == "UpgradeAvailable" then
      numPending = numPending + 1
      msg = msg .. ".status.state is '" .. obj.status.state .. "'\n"
    elseif obj.status.state == "UpgradePending" then
      -- UpgradePending with manual approval is expected behavior, treat as healthy
      if isManualApprovalPending then
        msg = msg .. ".status.state is 'AtLatestKnown'\n"
      else
        numPending = numPending + 1
        msg = msg .. ".status.state is '" .. obj.status.state .. "'\n"
      end
    elseif obj.status.state == "UpgradeFailed" then
      numDegraded = numDegraded + 1
      msg = msg .. ".status.state is '" .. obj.status.state .. "'\n"
    else
      -- Last possiblity of .status.state: AtLatestKnown
      msg =  msg .. ".status.state is '" .. obj.status.state .. "'\n"
    end
 
    if numDegraded == 0 and numPending == 0 then
      health_status.status = "Healthy"
      health_status.message = msg
      return health_status
    elseif numPending > 0 and numDegraded == 0 then
      health_status.status = "Progressing"
      health_status.message = msg
      return health_status
    else
      health_status.status = "Degraded"
      health_status.message = msg
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "An install plan for a subscription is pending installation"
return health_status
