local health_status = {}
if obj.status ~= nil then
  if obj.status.ingress ~= nil then
    -- Each router that serves the Route adds an ingress entry with an Admitted condition. Other controllers can add
    -- more Admitted conditions to an entry, so the entries are counted instead of the conditions.
    local numIngresses = 0
    local numAdmitted = 0
    local numRejected = 0
    for _, ingress in pairs(obj.status.ingress) do
      numIngresses = numIngresses + 1
      local admitted = false
      local rejected = false
      if ingress.conditions ~= nil then
        for _, condition in pairs(ingress.conditions) do
          if condition.type == "Admitted" and condition.status == "True" then
            admitted = true
          elseif condition.type == "Admitted" and condition.status == "False" then
            rejected = true
          end
        end
      end
      if rejected then
        numRejected = numRejected + 1
      elseif admitted then
        numAdmitted = numAdmitted + 1
      end
    end
    if numRejected > 0 then
      health_status.status = "Degraded"
      health_status.message = "Route is degraded"
      return health_status
    elseif numAdmitted == numIngresses then
      health_status.status = "Healthy"
      health_status.message = "Route is healthy"
      return health_status
    else
      health_status.status = "Progressing"
      health_status.message = "Route is still getting admitted"
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Route is still getting admitted"
return health_status
