hs = {}

if obj.spec.suspend == true then
    hs.status = "Suspended"
    hs.message = "CronJob is Suspended"
    return hs
end

if obj.status ~= nil then
    if obj.status.active ~= nil and table.getn(obj.status.active) > 0 then
        -- We could be Progressing very often, depending on the Cron schedule, which would bubble up
        -- to the Application health. If this is undesired, the annotation `argocd.argoproj.io/ignore-healthcheck: "true"`
        -- can be added on the CronJob.
        hs.status = "Progressing"
        hs.message = string.format("Waiting for %d Jobs to complete", table.getn(obj.status.active))
        return hs
    end

    -- If the CronJob has no active jobs and the lastSuccessfulTime < lastScheduleTime
    -- then we know it failed the last execution
    if obj.status.lastScheduleTime ~= nil then
        -- No issue comparing time as text
        if obj.status.lastSuccessfulTime == nil or obj.status.lastSuccessfulTime < obj.status.lastScheduleTime then
            hs.status = "Degraded"
            hs.message = "CronJob has not completed its last execution successfully"
            return hs
        end
        hs.message = "CronJob has completed its last execution successfully"
    end

    -- There is no way to know if as CronJob missed its execution based on status
    -- so we assume Healthy even if a cronJob is not getting scheduled.
    -- https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#job-creation
    hs.status = "Healthy"
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for CronJob"
return hs
