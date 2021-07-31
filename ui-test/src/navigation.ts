import {By, WebDriver} from 'selenium-webdriver';
import {ApplicationsList} from './applications-list/applications-list';
import UiTestUtilities from './UiTestUtilities';
import {Base} from './base';

const NAVBAR_APPLICATIONS_BUTTON: By = By.css('#app .nav-bar .argo-icon-application');
const NAVBAR_SETTINGS_BUTTON: By = By.css('#app .nav-bar .argo-icon-settings');
const NAVBAR_USER_INFO_BUTTON: By = By.css('#app .nav-bar .fa-user-circle');
const NAVBAR_DOCS_BUTTON: By = By.css('#app .nav-bar .argo-icon-docs');

export class Navigation extends Base {
    private applicationsList: ApplicationsList;

    public constructor(driver: WebDriver) {
        super(driver);
        this.applicationsList = new ApplicationsList(this.driver);
    }

    /**
     * Click the Applications Nav Bar Button
     * Return: reference to ApplicationsList page
     */
    public async clickApplicationsNavBarButton(): Promise<ApplicationsList> {
        try {
            const navBarButton = await UiTestUtilities.findUiElement(this.driver, NAVBAR_APPLICATIONS_BUTTON);
            await navBarButton.click();
        } catch (err) {
            throw new Error(err);
        }
        return this.applicationsList;
    }

    /**
     * Click the Settings Nav Bar Button
     * TODO return settings page
     */
    public async clickSettingsNavBarButton() {
        try {
            const navBarButton = await UiTestUtilities.findUiElement(this.driver, NAVBAR_SETTINGS_BUTTON);
            await navBarButton.click();
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Click the User Info Nav Bar Button
     * TODO return User Info page
     */
    public async clickUserInfoNavBarButton() {
        try {
            const navBarButton = await UiTestUtilities.findUiElement(this.driver, NAVBAR_USER_INFO_BUTTON);
            await navBarButton.click();
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Click the Documentation Nav Bar Button
     * TODO return docs page
     */
    public async clickDocsNavBarButton() {
        try {
            const navBarButton = await UiTestUtilities.findUiElement(this.driver, NAVBAR_DOCS_BUTTON);
            await navBarButton.click();
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Get the WebDriver. Test cases are not recommended to use this. Use Page/Component objects to perform actions
     */
    public getDriver(): WebDriver {
        return this.driver;
    }

    /**
     * Call when test case is finished
     */
    public async quit() {
        await this.driver.quit();
    }

    /**
     * Sleep for t milliseconds. This is not recommended for use by test cases.
     * @param t
     */
    public async sleep(t: number) {
        await this.driver.sleep(t);
    }
}
