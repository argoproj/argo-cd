import {By, until, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import UiTestUtilities from '../UiTestUtilities';
import * as Const from '../Constants';

const CREATE_APPLICATION_BUTTON_CREATE: By = By.xpath('.//button[@qe-id="applications-list-button-create"]');
const CREATE_APPLICATION_BUTTON_CANCEL: By = By.xpath('.//button[@qe-id="applications-list-button-cancel"]');

const CREATE_APPLICATION_FIELD_APP_NAME: By = By.xpath('.//input[@qeid="application-create-field-app-name"]');
const CREATE_APPLICATION_FIELD_PROJECT: By = By.xpath('.//input[@qe-id="application-create-field-project"]');
const CREATE_APPLICATION_FIELD_REPOSITORY_URL: By = By.xpath('.//input[@qe-id="application-create-field-repository-url"]');
const CREATE_APPLICATION_FIELD_REPOSITORY_PATH: By = By.xpath('.//input[@qe-id="application-create-field-path"]');

const CREATE_APPLICATION_DROPDOWN_DESTINATION: By = By.xpath('.//div[@qe-id="application-create-dropdown-destination"]');
const CREATE_APPLICATION_DROPDOWN_MENU_URL: By = By.xpath('.//li[@qe-id="application-create-dropdown-destination-URL"]');
const CREATE_APPLICATION_DROPDOWN_MENU_NAME: By = By.xpath('.//li[@qe-id="application-create-dropdown-destination-NAME"]');

export const DESTINATION_MENU_NAME: string = 'NAME';
export const DESTINATION_MENU_URL: string = 'URL';

const CREATE_APPLICATION_FIELD_CLUSTER_NAME: By = By.xpath('.//input[@qe-id="application-create-field-cluster-name"]');
const CREATE_APPLICATION_FIELD_CLUSTER_NAMESPACE: By = By.xpath('.//input[@qeid="application-create-field-namespace"]');
const CREATE_APPLICATION_FIELD_CLUSTER_URL: By = By.xpath('.//input[@qe-id="application-create-field-cluster-url"]');

export class ApplicationCreatePanel extends Base {
    public constructor(driver: WebDriver) {
        super(driver);
    }

    public async setAppName(appName: string): Promise<void> {
        try {
            const appNameField = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_FIELD_APP_NAME);
            await appNameField.sendKeys(appName);
        } catch (err) {
            throw new Error(err);
        }
    }

    public async setProjectName(projectName: string): Promise<void> {
        try {
            const project = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_FIELD_PROJECT);
            await project.sendKeys(projectName);
        } catch (err) {
            throw new Error(err);
        }
    }

    public async setSourceRepoUrl(sourceRepoUrl: string): Promise<void> {
        try {
            const reposUrl = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_FIELD_REPOSITORY_URL);
            await reposUrl.sendKeys(sourceRepoUrl);
        } catch (err) {
            throw new Error(err);
        }
    }

    public async setSourceRepoPath(sourceRepoPath: string): Promise<void> {
        try {
            const path = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_FIELD_REPOSITORY_PATH);
            await path.sendKeys(sourceRepoPath);
        } catch (err) {
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
            const clusterCombo = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_DROPDOWN_DESTINATION);
            // click() doesn't work. Use script
            await UiTestUtilities.click(this.driver, clusterCombo);
            const urlMenu = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_DROPDOWN_MENU_URL);
            await urlMenu.click();
            if (destinationClusterFieldValue) {
                await this.setDestinationClusterUrl(destinationClusterFieldValue);
            }
        } catch (err) {
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
            const clusterCombo = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_DROPDOWN_DESTINATION);
            // click() doesn't work. Use script
            await UiTestUtilities.click(this.driver, clusterCombo);
            const nameMenu = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_DROPDOWN_MENU_NAME);
            await UiTestUtilities.click(this.driver, nameMenu);
            if (destinationClusterFieldValue) {
                await this.setDestinationClusterName(destinationClusterFieldValue);
            }
        } catch (err) {
            throw new Error(err);
        }
    }

    public async setDestinationClusterName(destinationClusterName: string): Promise<void> {
        try {
            const clusterName = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_FIELD_CLUSTER_NAME);
            await clusterName.sendKeys(destinationClusterName);
            // await clusterName.sendKeys('\r');
        } catch (err) {
            throw new Error(err);
        }
    }

    public async setDestinationClusterUrl(destinationClusterUrl: string): Promise<void> {
        try {
            const clusterUrl = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_FIELD_CLUSTER_URL);
            await clusterUrl.sendKeys(destinationClusterUrl);
        } catch (err) {
            throw new Error(err);
        }
    }

    public async setDestinationNamespace(destinationNamespace: string): Promise<void> {
        try {
            const namespace = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_FIELD_CLUSTER_NAMESPACE);
            await namespace.sendKeys(destinationNamespace);
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Click the Create button to create the app
     */
    public async clickCreateButton(): Promise<void> {
        try {
            const createButton = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_BUTTON_CREATE);
            await createButton.click();

            // Wait until the Create Application Sliding Panel disappears
            await this.driver.wait(until.elementIsNotVisible(createButton), Const.TEST_SLIDING_PANEL_TIMEOUT).catch(e => {
                UiTestUtilities.logError('The Create Application Sliding Panel did not disappear');
                throw e;
            });
            await this.driver.sleep(1000);
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Click the Cancel Button.  Do not create the app.
     */
    public async clickCancelButton(): Promise<void> {
        try {
            const cancelButton = await UiTestUtilities.findUiElement(this.driver, CREATE_APPLICATION_BUTTON_CANCEL);
            await cancelButton.click();

            // Wait until the Create Application Sliding Panel disappears
            await this.driver.wait(until.elementIsNotVisible(cancelButton), Const.TEST_SLIDING_PANEL_TIMEOUT).catch(e => {
                UiTestUtilities.logError('The Create Application Sliding Panel did not disappear');
                throw e;
            });
        } catch (err) {
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
            await this.setAppName(appName);
            await this.setProjectName(projectName);
            await this.setSourceRepoUrl(sourceRepoUrl);
            await this.setSourceRepoPath(sourceRepoPath);
            await this.selectDestinationClusterNameMenu(destinationClusterName);
            await this.setDestinationNamespace(destinationNamespace);
            await this.clickCreateButton();
        } catch (err) {
            throw new Error(err);
        }
    }
}
