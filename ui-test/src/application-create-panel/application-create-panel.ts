import {By, until, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import UiTestUtilities from '../UiTestUtilities';
import Configuration from '../Configuration';

export class ApplicationCreatePanel extends Base {
    public static readonly BUTTON_CREATE: By = By.xpath('.//button[@qe-id="applications-list-button-create"]');
    public static readonly BUTTON_CANCEL: By = By.xpath('.//button[@qe-id="applications-list-button-cancel"]');

    public static readonly FIELD_APP_NAME: By = By.xpath('.//input[@qeid="application-create-field-app-name"]');
    public static readonly FIELD_PROJECT: By = By.xpath('.//input[@qe-id="application-create-field-project"]');
    public static readonly FIELD_REPOSITORY_URL: By = By.xpath('.//input[@qe-id="application-create-field-repository-url"]');
    public static readonly FIELD_REPOSITORY_PATH: By = By.xpath('.//input[@qe-id="application-create-field-path"]');

    public static readonly DROPDOWN_DESTINATION: By = By.xpath('.//div[@qe-id="application-create-dropdown-destination"]//p');
    public static readonly DROPDOWN_MENU_URL: By = By.xpath('.//li[@qe-id="application-create-dropdown-destination-URL"]');
    public static readonly DROPDOWN_MENU_NAME: By = By.xpath('.//li[@qe-id="application-create-dropdown-destination-NAME"]');

    public static readonly DESTINATION_MENU_NAME: string = 'NAME';
    public static readonly DESTINATION_MENU_URL: string = 'URL';

    public static readonly FIELD_CLUSTER_NAME: By = By.xpath('.//input[@qe-id="application-create-field-cluster-name"]');
    public static readonly FIELD_CLUSTER_NAMESPACE: By = By.xpath('.//input[@qeid="application-create-field-namespace"]');
    public static readonly FIELD_CLUSTER_URL: By = By.xpath('.//input[@qe-id="application-create-field-cluster-url"]');

    public constructor(driver: WebDriver) {
        super(driver);
    }

    public async setAppName(appName: string): Promise<void> {
        try {
            const appNameField = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.FIELD_APP_NAME);
            await appNameField.sendKeys(appName);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while setting app name: ' + err);
            throw new Error(err);
        }
    }

    public async setProjectName(projectName: string): Promise<void> {
        try {
            const project = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.FIELD_PROJECT);
            await project.sendKeys(projectName);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while setting project name: ' + err);
            throw new Error(err);
        }
    }

    public async setSourceRepoUrl(sourceRepoUrl: string): Promise<void> {
        try {
            const reposUrl = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.FIELD_REPOSITORY_URL);
            await reposUrl.sendKeys(sourceRepoUrl);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while setting source repo URL: ' + err);
            throw new Error(err);
        }
    }

    public async setSourceRepoPath(sourceRepoPath: string): Promise<void> {
        try {
            const path = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.FIELD_REPOSITORY_PATH);
            await path.sendKeys(sourceRepoPath);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while setting source repo path: ' + err);
            throw new Error(err);
        }
    }

    /**
     * Convenience method to select the Destination Cluster URL menu and set the url field with destinationClusterFieldValue
     *
     * @param destinationClusterFieldValue
     */
    public async selectDestinationClusterURLMenu(destinationClusterFieldValue: string): Promise<void> {
        try {
            await UiTestUtilities.sleep(200)
            const clusterCombo = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.DROPDOWN_DESTINATION);
            await clusterCombo.click();
            await UiTestUtilities.sleep(10);
            const urlMenu = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.DROPDOWN_MENU_URL);
            await urlMenu.click();
            if (destinationClusterFieldValue) {
                await this.setDestinationClusterUrl(destinationClusterFieldValue);
            }
        } catch (err: any) {
            UiTestUtilities.log('Error caught while selecting destination cluster URL menu: ' + err);
            throw new Error(err);
        }
    }

    /**
     * Convenience method to select the Destination Cluster Name menu and set the namefield with destinationClusterFieldValue
     *
     * @param destinationClusterFieldValue
     */
    public async selectDestinationClusterNameMenu(destinationClusterFieldValue: string): Promise<void> {
        try {
            await UiTestUtilities.sleep(200)
            const clusterCombo = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.DROPDOWN_DESTINATION);
            await clusterCombo.click();
            await UiTestUtilities.sleep(10);
            const nameMenu = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.DROPDOWN_MENU_NAME);
            await nameMenu.click();

            await this.setDestinationClusterName(destinationClusterFieldValue);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while selecting destination cluster name menu: ' + err);
            throw new Error(err);
        }
    }

    public async setDestinationClusterName(destinationClusterName: string): Promise<void> {
        try {
            const clusterName = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.FIELD_CLUSTER_NAME);
            await clusterName.sendKeys(destinationClusterName);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while setting destination cluster name: ' + err);
            throw new Error(err);
        }
    }

    public async setDestinationClusterUrl(destinationClusterUrl: string): Promise<void> {
        try {
            const clusterUrl = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.FIELD_CLUSTER_URL);
            await clusterUrl.sendKeys(destinationClusterUrl);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while setting destination cluster URL: ' + err);
            throw new Error(err);
        }
    }

    public async setDestinationNamespace(destinationNamespace: string): Promise<void> {
        try {
            const namespace = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.FIELD_CLUSTER_NAMESPACE);
            await namespace.sendKeys(destinationNamespace);
        } catch (err: any) {
            UiTestUtilities.log('Error caught while setting destination namespace: ' + err);
            throw new Error(err);
        }
    }

    /**
     * Click the Create button to create the app
     */
    public async clickCreateButton(): Promise<void> {
        try {
            const createButton = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.BUTTON_CREATE);
            await createButton.click();
            const formErrors: string = await UiTestUtilities.getFormErrors(this.driver)
            if (formErrors) {
                throw Error(`Error From Form: ${formErrors}`)
            }
            const toastError: string = await UiTestUtilities.getErrorToast(this.driver)
            if (toastError) {
                throw Error(`Error from Toast: ${toastError}`)
            }
            await UiTestUtilities.sleep(200)
            await UiTestUtilities.captureSession(this.driver, "clickCreateButton_after.png")
            await this.driver.wait(until.elementIsNotVisible(createButton), Configuration.TEST_TIMEOUT).catch((e) => {
                UiTestUtilities.logError('The Create Application Sliding Panel did not disappear');
                UiTestUtilities.captureSession(this.driver, "clickCreateButton_after_notdisapeared.png")
                throw e;
            });
        } catch (err: any) {
            UiTestUtilities.log('Error caught while clicking Create button: ' + err);
            throw new Error(err);
        }
    }

    /**
     * Click the Cancel Button.  Do not create the app.
     */
    public async clickCancelButton(): Promise<void> {
        try {
            const cancelButton = await UiTestUtilities.findUiElement(this.driver, ApplicationCreatePanel.BUTTON_CANCEL);
            await cancelButton.click();

            // Wait until the Create Application Sliding Panel disappears
            await this.driver.wait(until.elementIsNotVisible(cancelButton), Configuration.TEST_SLIDING_PANEL_TIMEOUT).catch((e) => {
                UiTestUtilities.logError('The Create Application Sliding Panel did not disappear');
                throw e;
            });
        } catch (err: any) {
            UiTestUtilities.log('Error caught while clicking Cancel button: ' + err);
            throw new Error(err);
        }
    }

    /**
     * Convenience method to create an application given the following inputs to the dialog
     *
     * TODO add Sync Policy and Sync Options and setter methods above
     *
     * @param appName
     * @param projectName
     * @param sourceRepoUrl
     * @param sourceRepoPath
     * @param destinationMenu
     * @param destinationClusterName
     * @param destinationNamespace
     */
    public async createApplication(
        appName: string,
        projectName: string,
        sourceRepoUrl: string,
        sourceRepoPath: string,
        destinationClusterName: string,
        destinationNamespace: string
    ): Promise<void> {
        UiTestUtilities.log('About to create application');
        try {
            await this.selectDestinationClusterNameMenu(destinationClusterName);
            await this.setAppName(appName);
            await this.setProjectName(projectName);
            await this.setSourceRepoUrl(sourceRepoUrl);
            await this.setSourceRepoPath(sourceRepoPath);
            await this.setDestinationNamespace(destinationNamespace);

            UiTestUtilities.log('Clicking on create!');
            await this.clickCreateButton();
        } catch (err: any) {
            throw new Error(err);
        }
    }
}
