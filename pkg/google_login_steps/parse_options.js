const fs = require('fs');
const html = fs.readFileSync('account_login_validation_try_another_way_dump.html', 'utf8');
const { JSDOM } = require('jsdom');
const dom = new JSDOM(html);
const document = dom.window.document;

Array.from(document.querySelectorAll('div, li, button, span, a')).forEach(el => {
    if (el.textContent && el.textContent.toLowerCase().includes('passkey') && el.children.length < 5) {
        console.log({
            tag: el.tagName,
            text: el.textContent.trim(),
            role: el.getAttribute('role'),
            classes: el.className
        });
    }
});
