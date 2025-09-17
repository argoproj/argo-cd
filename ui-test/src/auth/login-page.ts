import {By, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import Configuration from '../Configuration';
import UiTestUtilities from '../UiTestUtilities';

const LOGIN_FORM: By = By.css('#app .login__box form');
const LOGIN_FORM_INPUT: By = By.css('input.argo-field');
const LOGIN_FORM_BUTTON: By = By.css('button.argo-button');

export class AuthLoginPage extends Base {
    public constructor(driver: WebDriver) {
        super(driver);
    }

    /**
     * Fill login form and submit it
     */
    public async loginWithCredentials() {
        const loginForm = await UiTestUtilities.findUiElement(this.driver, LOGIN_FORM);
        const inputs = await loginForm.findElements(LOGIN_FORM_INPUT);
        const submitButton = await loginForm.findElement(LOGIN_FORM_BUTTON);

        await inputs[0].sendKeys(Configuration.ARGOCD_AUTH_USERNAME);
        await inputs[1].sendKeys(Configuration.ARGOCD_AUTH_PASSWORD);

        await submitButton.click();
    }
}
