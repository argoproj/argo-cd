require('dotenv').config({path: __dirname + '/../.env'});

export default class Configuration {
    // Test configuration
    public static readonly ACCEPT_INSECURE_CERTS: string | undefined = process.env.ACCEPT_INSECURE_CERTS;
    public static readonly ENABLE_CONSOLE_LOG: boolean = (process.env.ENABLE_CONSOLE_LOG ?? "true") !== "false";
    public static readonly TEST_TIMEOUT: number = parseInt(process.env.TEST_TIMEOUT ?? "60000");
    public static readonly TEST_ERROR_TOAST_TIMEOUT: number = parseInt(process.env.TEST_ERROR_TOAST_TIMEOUT ?? "1000");
    public static readonly TEST_IS_NOT_VISIBLE_TIMEOUT: number = parseInt(process.env.TEST_IS_NOT_VISIBLE_TIMEOUT ?? "10000");
    public static readonly TEST_SLIDING_PANEL_TIMEOUT: number = parseInt(process.env.TEST_SLIDING_PANEL_TIMEOUT ?? "5000");
    public static readonly TEST_SCREENSHOTS_DIRECTORY: string = process.env.TEST_SCREENSHOTS_DIRECTORY ?? "/root/.npm/_logs/";

    // ArgoCD UI specific.  These are for single application-based tests, so one can quickly create an app based on the environment variables
    public static readonly ARGOCD_URL: string = process.env.ARGOCD_URL ? process.env.ARGOCD_URL : '';
    public static readonly ARGOCD_NAMESPACE: string = process.env.ARGOCD_NAMESPACE || 'argocd';
    public static readonly ARGOCD_AUTH_USERNAME: string = process.env.ARGOCD_AUTH_USERNAME || '';
    public static readonly ARGOCD_AUTH_PASSWORD: string = process.env.ARGOCD_AUTH_PASSWORD || '';
    public static readonly APP_NAME: string = process.env.APP_NAME ? process.env.APP_NAME : '';
    public static readonly APP_PROJECT: string = process.env.APP_PROJECT ? process.env.APP_PROJECT : '';
    public static readonly GIT_REPO: string = process.env.GIT_REPO ? process.env.GIT_REPO : '';
    public static readonly SOURCE_REPO_PATH: string = process.env.SOURCE_REPO_PATH ? process.env.SOURCE_REPO_PATH : '';
    public static readonly DESTINATION_CLUSTER_NAME: string = process.env.DESTINATION_CLUSTER_NAME ? process.env.DESTINATION_CLUSTER_NAME : '';
    public static readonly DESTINATION_NAMESPACE: string = process.env.DESTINATION_NAMESPACE ? process.env.DESTINATION_NAMESPACE : '';
}
