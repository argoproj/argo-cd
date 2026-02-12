import Configuration from './Configuration';
import {Builder, By, until, WebDriver, WebElement} from 'selenium-webdriver';
import chrome from 'selenium-webdriver/chrome';
import * as fs from 'fs';
import {Navigation} from './navigation';

export default class UiTestUtilities {
    /**
     * Log a message to the console.
     * @param message
     */
    public static async log(message: string): Promise<void> {
        if (Configuration.ENABLE_CONSOLE_LOG) {
            // tslint:disable-next-line:no-console
            console.log(message);
        }
    }

    public static async logError(message: string): Promise<void> {
        if (Configuration.ENABLE_CONSOLE_LOG) {
            // tslint:disable-next-line:no-console
            console.error(message);
        }
    }

    /**
     * Set up the WebDriver. Initial steps for all tests.  Returns the instance of Navigation with the WebDriver.
     * From there, navigate the UI.  Test cases do no need to reference the instance of WebDriver since Component/Page-specific
     * API methods should be called instead.
     *
     */
    public static async init(): Promise<Navigation> {
        const options = new chrome.Options();
        if (process.env.ACCEPT_INSECURE_CERTS !== 'false') {
            options.setAcceptInsecureCerts(true);
        }
        if (process.env.IS_HEADLESS !== 'false') {
            UiTestUtilities.log('Adding headless option');
            options.addArguments('headless');
        }
        options.addArguments(
            'window-size=1400,800',
            "enable-javascript",
            "no-sandbox",
            "disable-gpu",
            "disable-dev-shm-usage"
        );
        const driver = await new Builder().forBrowser('chrome').setChromeOptions(options).build();

        UiTestUtilities.log('Environment variables are:');
        const loadedConfig = require('dotenv').config({path: __dirname + '/../.env'});
        if (loadedConfig && loadedConfig.parsed && loadedConfig.parsed["ARGOCD_AUTH_PASSWORD"] !== undefined) {
            loadedConfig.parsed["ARGOCD_AUTH_PASSWORD"] = "<REDACTED>";
        }
        UiTestUtilities.log(loadedConfig);

        // Navigate to the ArgoCD URL
        await driver.get(Configuration.ARGOCD_URL);
        UiTestUtilities.log('Navigate to Argo CD URL successful: driver.get');
        return new Navigation(driver);
    }

    /**
     *  Ensure that the given string is in the page.
     */
    public static async existInPage(driver: WebDriver, text: string): Promise<boolean> {
        return (await driver.getPageSource()).includes(text);
    }

    /**
     * Take a screenshot of the current session and save it to the directory mounted on `_logs`.
     *
     * @param driver
     * @param fileName
     */
    public static async captureSession(driver: any, fileName: string) {
        const screenshot = await driver.takeScreenshot();
        const binaryBuffer: Buffer = Buffer.from(screenshot, 'base64');
        fs.writeFileSync(`${Configuration.TEST_SCREENSHOTS_DIRECTORY}/${fileName}`, binaryBuffer);
        console.log(`Screenshot saved to: ${fileName}`);
    }

    /**
     * Locate the UI Element for the given locator, and wait until it is visible
     *
     * @param driver
     * @param locator
     */
    public static async findUiElement(driver: WebDriver, locator: By, timeout: number = Configuration.TEST_TIMEOUT): Promise<WebElement> {
        try {
            const element = await driver.wait(until.elementLocated(locator), timeout);
            const isDisplayed = await element.isDisplayed();
            if (isDisplayed) {
                await driver.wait(until.elementIsVisible(element), timeout);
            }
            return element;
        } catch (err) {
            throw err;
        }
    }

    public static async getErrorToast(driver: WebDriver): Promise<string> {
        const ERROR_TOAST_LOCATOR = By.xpath("//div[contains(@class, 'Toastify__toast-body')]//span");
        const toastSpan = await driver.wait(
            until.elementLocated(ERROR_TOAST_LOCATOR),
            Configuration.TEST_ERROR_TOAST_TIMEOUT
        ).catch((_) => { return null });
        if (toastSpan) {
            const text = await toastSpan.getAttribute('innerHTML');
            return text.trim();
        }
        return "";
    }

    public static async getFormErrors(driver: WebDriver): Promise<string> {
        const ERROR_FORM_LOCATOR = By.xpath("//div[contains(@class, 'argo-form-row__error-msg')]");
        const formErrors = await driver.wait(
            until.elementLocated(ERROR_FORM_LOCATOR),
            Configuration.TEST_ERROR_TOAST_TIMEOUT
        ).catch((_) => { return null });
        if (formErrors) {
            const text = await formErrors.getAttribute('innerHTML');
            return text.trim();
        }
        return "";
    }

    /**
     * Similar to until.methods and used in driver.wait, this will wait until
     * the expected attribute is the same as the actual attribute on the element
     *
     * @param attr
     * @param attrValue
     */
    public static async untilAttributeIs(element: WebElement, attr: string, attrValue: string): Promise<boolean> {
        const actual = await element.getAttribute(attr);
        UiTestUtilities.log('Actual = ' + actual + ', expected = ' + attrValue + ', ' + (actual === attrValue));
        return actual === attrValue;
    }

    /**
    * Wait for the given amount of time.
    *
    * @param milliseconds
     */
    public static async sleep(milliseconds: number) {
        await new Promise(f => setTimeout(f, milliseconds));
    }

}
