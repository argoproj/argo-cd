local health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local numDegraded = 0
    local numPending = 0
    local msg = ""
    for i, condition in pairs(obj.status.conditions) do
      msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. "\n"
      if condition.type == "InstallPlanPending" and condition.status == "True" then
        numPending = numPending + 1
      elseif (condition.type == "InstallPlanMissing" and condition.reason ~= "ReferencedInstallPlanNotFound") then
        numDegraded = numDegraded + 1
      elseif (condition.type == "CatalogSourcesUnhealthy" or condition.type == "InstallPlanFailed" or condition.type == "ResolutionFailed") and condition.status == "True" then
        numDegraded = numDegraded + 1
      end
    end

    -- Available states: undef/nil, UpgradeAvailable, UpgradePending, UpgradeFailed, AtLatestKnown
    -- Source: https://github.com/openshift/operator-framework-olm/blob/5e2c73b7663d0122c9dc3e59ea39e515a31e2719/staging/api/pkg/operators/v1alpha1/subscription_types.go#L17-L23
    if obj.status.state == nil  then
      numPending = numPending + 1
      msg = msg .. ".status.state not yet known\n"
    elseif obj.status.state == "" or obj.status.state == "UpgradeAvailable" or obj.status.state == "UpgradePending" then
      numPending = numPending + 1
      msg = msg .. ".status.state is '" .. obj.status.state .. "'\n"
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
