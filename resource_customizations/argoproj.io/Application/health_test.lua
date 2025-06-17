tests = {
    {
        given = {
            status = {
                health = {
                    status = "Healthy",
                    message = "Application is healthy"
                }
            }
        },
        want = {
            status = "Healthy",
            message = "Application is healthy"
        }
    },
    {
        given = {
            status = {
                health = {
                    status = "Degraded",
                    message = "Application has issues"
                }
            }
        },
        want = {
            status = "Degraded",
            message = "Application has issues"
        }
    },
    {
        given = {},
        want = {
            status = "Progressing",
            message = "Waiting for application to be reconciled"
        }
    }
}

for _, test in ipairs(tests) do
    local state = health(test.given)
    if state.status ~= test.want.status then
        error(string.format("Expected status %s but got %s", test.want.status, state.status))
    end
    if state.message ~= test.want.message then
        error(string.format("Expected message '%s' but got '%s'", test.want.message, state.message))
    end
end