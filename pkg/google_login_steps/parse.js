const fs = require('fs');
const html = fs.readFileSync('account_login_step1_dropdown_dump.html', 'utf8');
const textMatches = html.match(/>([^<]+)</g);
if (textMatches) {
  const text = textMatches.map(t => t.slice(1, -1).trim()).filter(t => t.length > 0);
  console.log([...new Set(text)].join('\n'));
}
