async function test() {
  const res = await fetch('http://127.0.0.1:8080/api/telegram/quick-order', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': '123456',
    },
    body: JSON.stringify({ mode: 'price', planCode: '24adv01-v3', datacenter: 'sbg' }),
  });
  const data = await res.json();
  console.log(data);
}
test();
