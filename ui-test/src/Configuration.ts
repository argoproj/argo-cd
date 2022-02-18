require('dotenv').config({path: __dirname + '/.env'});

export default class Configuration {
    // Test specific
    public static readonly ENABLE_CONSOLE_LOG: string | undefined = process.env.ENABLE_CONSOLE_LOG;
    public static readonly TEST_TIMEOUT: string | undefined = process.env.TEST_TIMEOUT;
    // ArgoCD UI specific.  These are for single application-based tests, so one can quickly create an app based on the environment variables
    public static readonly ARGOCD_URL: string = process.env.ARGOCD_URL ? process.env.ARGOCD_URL : '';
    public static readonly APP_NAME: string = process.env.APP_NAME ? process.env.APP_NAME : '';
    public static readonly APP_PROJECT: string = process.env.APP_PROJECT ? process.env.APP_PROJECT : '';
    public static readonly GIT_REPO: string = process.env.GIT_REPO ? process.env.GIT_REPO : '';
    public static readonly SOURCE_REPO_PATH: string = process.env.SOURCE_REPO_PATH ? process.env.SOURCE_REPO_PATH : '';
    public static readonly DESTINATION_CLUSTER_NAME: string = process.env.DESTINATION_CLUSTER_NAME ? process.env.DESTINATION_CLUSTER_NAME : '';
    public static readonly DESTINATION_NAMESPACE: string = process.env.DESTINATION_NAMESPACE ? process.env.DESTINATION_NAMESPACE : '';
}
