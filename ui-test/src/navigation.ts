import {By, WebDriver} from 'selenium-webdriver';
import {ApplicationsList} from './applications-list/applications-list';
import UiTestUtilities from './UiTestUtilities';
import {Base} from './base';

const NAVBAR_APPLICATIONS_BUTTON: By = By.css('#app .sidebar .argo-icon-application');
const NAVBAR_SETTINGS_BUTTON: By = By.css('#app .sidebar .argo-icon-settings');
const NAVBAR_USER_INFO_BUTTON: By = By.css('#app .sidebar .fa-user-circle');
const NAVBAR_DOCS_BUTTON: By = By.css('#app .sidebar .argo-icon-docs');

const LOGIN_USERNAME_INPUT: By = By.css('.login input');
const LOGIN_PASSWORD_INPUT: By = By.css('.login input[type="password"]');

export class Navigation extends Base {
    private applicationsList: ApplicationsList;

    public constructor(driver: WebDriver) {
        super(driver);
        this.applicationsList = new ApplicationsList(this.driver);
    }

    public async login(username: string, password: string): Promise<void> {
        try {
            const usernameInput = await UiTestUtilities.findUiElement(this.driver, LOGIN_USERNAME_INPUT);
            await usernameInput.sendKeys(username);

            const passwordInput = await UiTestUtilities.findUiElement(this.driver, LOGIN_PASSWORD_INPUT);
            await passwordInput.sendKeys(password);

            await passwordInput.submit();
        } catch (err) {
            throw new Error("Error logging in: " + err);
        }
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
            throw new Error("Error clicking applications nav bar button: " + err);
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
            throw new Error("Error clicking settings nav bar button: " + err);
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
            throw new Error("Error clicking user info nav bar button: " + err);
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
            throw new Error("Error clicking docs nav bar button: " + err);
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
