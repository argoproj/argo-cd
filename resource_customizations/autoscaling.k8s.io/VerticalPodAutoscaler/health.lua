-- Health check for the VerticalPodAutoscaler CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the VPA status conditions to an Argo CD health status:
--   Degraded    - NoPodsMatched is True (the selector matches no pods).
--   Healthy     - RecommendationProvided is True (a recommendation is available).
--   Progressing - the recommender has not produced a recommendation yet, or
--                 status is not populated yet.
local hs = { status = "Progressing", message = "Waiting for VPA recommendation" }
if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "NoPodsMatched" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or condition.reason or "No pods match the VPA selector"
      return hs
    end
  end
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "RecommendationProvided" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = "VPA recommendation provided"
      return hs
    end
  end
end
return hs
