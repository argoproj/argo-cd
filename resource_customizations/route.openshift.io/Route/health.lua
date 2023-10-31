health_status = {}
if obj.status ~= nil then
  if obj.status.ingress ~= nil then
    numIngressRules = 0
    numTrue = 0
    numFalse = 0
    for _, ingressRules in pairs(obj.status.ingress) do
        numIngressRules = numIngressRules + 1
        if obj.status.ingress ~= nil then
          for _, condition in pairs(ingressRules.conditions) do
              if condition.type == "Admitted" and condition.status == "True" then
                  numTrue = numTrue + 1
              elseif condition.type == "Admitted" and condition.status == "False" then
                  numFalse = numFalse + 1
              end
          end
        end
        health_status.status = 'Test'
    end
    if numTrue == numIngressRules then
      health_status.status = "Healthy"
      health_status.message = "Route is healthy"
      return health_status
    elseif numFalse > 0 then
      health_status.status = "Degraded"
      health_status.message = "Route is degraded"
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