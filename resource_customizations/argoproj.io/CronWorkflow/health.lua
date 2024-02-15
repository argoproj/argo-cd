local hs = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "SpecError" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      elseif condition.type == "SubmissionError" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      elseif condition.type == "Completed" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      end
    end
  end
end

hs.status = "Healthy"
hs.message = "CronWorkflow is healthy."

return hs
